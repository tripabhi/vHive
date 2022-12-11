// MIT License
//
// Copyright (c) 2020 Dmitrii Ustiugov, Plamen Petrov and EASE lab
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package ctriface

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/cio"
	"github.com/containerd/containerd/namespaces"
	"github.com/containerd/containerd/oci"
	"github.com/containerd/containerd/remotes/docker"

	"github.com/firecracker-microvm/firecracker-containerd/proto" // note: from the original repo
	"github.com/firecracker-microvm/firecracker-containerd/runtime/firecrackeroci"
	"github.com/pkg/errors"

	_ "google.golang.org/grpc/codes"  //tmp
	_ "google.golang.org/grpc/status" //tmp

	"github.com/go-multierror/multierror"
	"github.com/vhive-serverless/vhive/memory/manager"
	"github.com/vhive-serverless/vhive/metrics"
	"github.com/vhive-serverless/vhive/misc"

	_ "github.com/davecgh/go-spew/spew" //tmp
)

// StartVMResponse is the response returned by StartVM
// TODO: Integrate response with non-cri API
type StartVMResponse struct {
	// GuestIP is the IP of the guest MicroVM
	GuestIP string
	FCPid string
}

// ghcr.io/ease-lab/helloworld:var_workload
// ghcr.io/ease-lab/pyaes:var_workload
// docker.io/vhiveease/video-analytics-recog:latest
// docker.io/lyuze/helloworld:0.2
const (
	testImageName = "docker.io/lyuze/helloworld:0.2"
	testImageNameVictim = "ghcr.io/ease-lab/helloworld:var_workload"
)

// StartVM Boots a VM if it does not exist
func (o *Orchestrator) StartVM(ctx context.Context, vmID, imageName string) (_ *StartVMResponse, _ *metrics.Metric, retErr error) {
	var (
		startVMMetric *metrics.Metric = metrics.NewMetric()
		tStart        time.Time
	)

	logger := log.WithFields(log.Fields{"vmID": vmID, "image": imageName})
	logger.Debug("StartVM: Received StartVM")

	vm, err := o.vmPool.Allocate(vmID, o.hostIface)
	if err != nil {
		logger.Error("failed to allocate VM in VM pool")
		return nil, nil, err
	}
	vm.MemSizeMib = 1024

	defer func() {
		// Free the VM from the pool if function returns error
		if retErr != nil {
			if err := o.vmPool.Free(vmID); err != nil {
				logger.WithError(err).Errorf("failed to free VM from pool after failure")
			}
		}
	}()

	ctx = namespaces.WithNamespace(ctx, namespaceName)
	tStart = time.Now()
	if vm.Image, err = o.getImage(ctx, imageName); err != nil {
		return nil, nil, errors.Wrapf(err, "Failed to get/pull image")
	}
	startVMMetric.MetricMap[metrics.GetImage] = metrics.ToUS(time.Since(tStart))

	tStart = time.Now()
	conf := o.getVMConfig(vm)
	resp, err := o.fcClient.CreateVM(ctx, conf)
	startVMMetric.MetricMap[metrics.FcCreateVM] = metrics.ToUS(time.Since(tStart))
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to create the microVM in firecracker-containerd")
	}

	defer func() {
		if retErr != nil {
			if _, err := o.fcClient.StopVM(ctx, &proto.StopVMRequest{VMID: vmID}); err != nil {
				logger.WithError(err).Errorf("failed to stop firecracker-containerd VM after failure")
			}
		}
	}()

	logger.Debug("StartVM: Creating a new container")
	tStart = time.Now()
	container, err := o.client.NewContainer(
		ctx,
		vmID,
		containerd.WithSnapshotter(o.snapshotter),
		containerd.WithNewSnapshot(vmID, *vm.Image),
		containerd.WithNewSpec(
			oci.WithImageConfig(*vm.Image),
			firecrackeroci.WithVMID(vmID),
			firecrackeroci.WithVMNetwork,
		),
		containerd.WithRuntime("aws.firecracker", nil),
	)
	startVMMetric.MetricMap[metrics.NewContainer] = metrics.ToUS(time.Since(tStart))
	vm.Container = &container
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to create a container")
	}

	defer func() {
		if retErr != nil {
			if err := container.Delete(ctx, containerd.WithSnapshotCleanup); err != nil {
				logger.WithError(err).Errorf("failed to delete container after failure")
			}
		}
	}()

	iologger := NewWorkloadIoWriter(vmID)
	o.workloadIo.Store(vmID, &iologger)
	logger.Debug("StartVM: Creating a new task")
	tStart = time.Now()
	task, err := container.NewTask(ctx, cio.NewCreator(cio.WithStreams(os.Stdin, iologger, iologger)))
	startVMMetric.MetricMap[metrics.NewTask] = metrics.ToUS(time.Since(tStart))
	vm.Task = &task
	if err != nil {
		return nil, nil, errors.Wrapf(err, "failed to create a task")
	}

	defer func() {
		if retErr != nil {
			if _, err := task.Delete(ctx); err != nil {
				logger.WithError(err).Errorf("failed to delete task after failure")
			}
		}
	}()

	logger.Debug("StartVM: Waiting for the task to get ready")
	tStart = time.Now()
	ch, err := task.Wait(ctx)
	startVMMetric.MetricMap[metrics.TaskWait] = metrics.ToUS(time.Since(tStart))
	vm.TaskCh = ch
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to wait for a task")
	}

	defer func() {
		if retErr != nil {
			if err := task.Kill(ctx, syscall.SIGKILL); err != nil {
				logger.WithError(err).Errorf("failed to kill task after failure")
			}
		}
	}()

	logger.Debug("StartVM: Starting the task")
	tStart = time.Now()
	if err := task.Start(ctx); err != nil {
		return nil, nil, errors.Wrap(err, "failed to start a task")
	}
	startVMMetric.MetricMap[metrics.TaskStart] = metrics.ToUS(time.Since(tStart))

	time.Sleep(1 * time.Second) // let the function run 1 sec

	// kill the process and get the exit status
	// log.Info("killing task...")
	// if err := task.Kill(ctx, syscall.SIGKILL); err != nil {
	// 	log.Info("Error killing task!")
	// }

	// // wait for the process to fully exit and print out the exit status

	// // log.Info("waiting for signal")
	// status := <-ch
	// // log.Info("Getting the code...")
	// code, _, err := status.Result()
	// if err != nil {
	// 	logger.WithError(err).Errorf("failed to kill task after failure")
	// }
	// log.Info("Exited with status: ", code)

	defer func() {
		if retErr != nil {
			if err := task.Kill(ctx, syscall.SIGKILL); err != nil {
				logger.WithError(err).Errorf("failed to kill task after failure")
			}
		}
	}()

	if err := os.MkdirAll(o.getVMBaseDir(vmID), 0777); err != nil {
		logger.Error("Failed to create VM base dir")
		return nil, nil, err
	}
	if o.GetUPFEnabled() {
		logger.Debug("Registering VM with the memory manager")

		stateCfg := manager.SnapshotStateCfg{
			VMID:             vmID,
			GuestMemPath:     o.getMemoryFile(vmID),
			BaseDir:          o.getVMBaseDir(vmID),
			GuestMemSize:     int(conf.MachineCfg.MemSizeMib) * 1024 * 1024,
			IsLazyMode:       o.isLazyMode,
			VMMStatePath:     o.getSnapshotFile(vmID),
			WorkingSetPath:   o.getWorkingSetFile(vmID),
			InstanceSockAddr: resp.UPFSockPath,
		}
		if err := o.memoryManager.RegisterVM(stateCfg); err != nil {
			return nil, nil, errors.Wrap(err, "failed to register VM with memory manager")
			// NOTE (Plamen): Potentially need a defer(DeregisteVM) here if RegisterVM is not last to execute
		}
	}

	logger.Debug("Successfully started a VM")

	return &StartVMResponse{GuestIP: vm.Ni.PrimaryAddress}, startVMMetric, nil
}

// StartVM Boots a VM if it does not exist
func (o *Orchestrator) StartVMModified(ctx context.Context, vmID, imageName string, vmSize, vCPUCount uint32) (response *StartVMResponse, _ *metrics.Metric, retErr error) {
	var (
		startVMMetric *metrics.Metric = metrics.NewMetric()
		tStart        time.Time
	)

	logger := log.WithFields(log.Fields{"vmID": vmID, "image": imageName})
	logger.Debug("StartVM: Received StartVM")

	vm, err := o.vmPool.Allocate(vmID, o.hostIface)
	if err != nil {
		logger.Error("failed to allocate VM in VM pool")
		return nil, nil, err
	}
	vm.VCPUCount = vCPUCount
	vm.MemSizeMib = vmSize

	defer func() {
		// Free the VM from the pool if function returns error
		if retErr != nil {
			if err := o.vmPool.Free(vmID); err != nil {
				logger.WithError(err).Errorf("failed to free VM from pool after failure")
			}
		}
	}()

	ctx = namespaces.WithNamespace(ctx, namespaceName)
	// tStart = time.Now()
	// log.Info("image name: ", imageName)
	if vm.Image, err = o.getImage(ctx, imageName); err != nil {
		return nil, nil, errors.Wrapf(err, "Failed to get/pull image")
	}
	// startVMMetric.MetricMap[metrics.GetImage] = metrics.ToUS(time.Since(tStart))

	// log.Info("start FcCreateVM...")
	// readInSectorBeforeRun, writeInSectorBeforeRun := getDiskStats()
	tStart = time.Now()
	conf := o.getVMConfig(vm)
	resp, err := o.fcClient.CreateVM(ctx, conf)
	startVMMetric.MetricMap[metrics.FcCreateVM] = metrics.ToUS(time.Since(tStart))
	if err != nil {
		log.Errorf("failed to delete container after failure: %v", err)
		return nil, nil, errors.Wrap(err, "failed to create the microVM in firecracker-containerd")
	}
	FCPid := resp.GetFirecrackerPID()
	response = &StartVMResponse{GuestIP: vm.Ni.PrimaryAddress, FCPid: FCPid}
	//set victim CPU affinity
	// if isVictim {
	// log.Info("setting ", FCPid, " to CPU ", vmID)
	{
		TaskSetCmd := fmt.Sprintf("taskset -cp %s %s", vmID, FCPid)
		TaskSetCmdExec := exec.Command("sudo", "/bin/bash", "-c", TaskSetCmd)
		stdout, err := TaskSetCmdExec.Output()
		if err != nil {
			fmt.Println(err.Error())
			return
		}
		fmt.Println(string(stdout))
	}
	// get disk stats after run
	// readInSectorAfterRun, writeInSectorAfterRun := getDiskStats()
	// log.Info("Read in MB: ", (readInSectorAfterRun - readInSectorBeforeRun)*512/1024/1024)
	// log.Info("Write in MB: ", (writeInSectorAfterRun - writeInSectorBeforeRun)*512/1024/1024)

	defer func() {
		if retErr != nil {
			if _, err := o.fcClient.StopVM(ctx, &proto.StopVMRequest{VMID: vmID}); err != nil {
				logger.WithError(err).Errorf("failed to stop firecracker-containerd VM after failure")
			}
		}
	}()

	log.Info("NewContainer for vmid: ", vmID)
	// {
	// 	ExecCmd := exec.Command("sudo", "/bin/bash", "-c", fmt.Sprintf("lsof -p %s", FCPid))
	// 	stdout, err := ExecCmd.Output()
	// 	if err != nil {
	// 		fmt.Println(err.Error())
	// 		return
	// 	}
	// 	fmt.Println(string(stdout))
	// }
	tStart = time.Now()
	container, err := o.client.NewContainer(
		ctx,
		vmID,
		containerd.WithSnapshotter(o.snapshotter),
		containerd.WithNewSnapshot(vmID, *vm.Image),
		containerd.WithNewSpec(
			oci.WithImageConfig(*vm.Image),
			firecrackeroci.WithVMID(vmID),
			firecrackeroci.WithVMNetwork,
		),
		containerd.WithRuntime("aws.firecracker", nil),
	)
	startVMMetric.MetricMap[metrics.NewContainer] = metrics.ToUS(time.Since(tStart))
	vm.Container = &container
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to create a container")
	}
	// readInSectorAfterRun, writeInSectorAfterRun = getDiskStats()
	// log.Info("Read in MB: ", (readInSectorAfterRun - readInSectorBeforeRun)*512/1024/1024)
	// log.Info("Write in MB: ", (writeInSectorAfterRun - writeInSectorBeforeRun)*512/1024/1024)
	// {
	// 	ExecCmd := exec.Command("sudo", "/bin/bash", "-c", fmt.Sprintf("lsof -p %s", FCPid))
	// 	stdout, err := ExecCmd.Output()
	// 	if err != nil {
	// 		fmt.Println(err.Error())
	// 		return
	// 	}
	// 	fmt.Println(string(stdout))
	// }
	defer func() {
		if retErr != nil {
			if err := container.Delete(ctx, containerd.WithSnapshotCleanup); err != nil {
				logger.WithError(err).Errorf("failed to delete container after failure")
			}
		}
	}()

	iologger := NewWorkloadIoWriter(vmID)
	o.workloadIo.Store(vmID, &iologger)
	// logger.Debug("StartVM: Creating a new task")
	// log.Info("NewTask for vmid: ", vmID)
	tStart = time.Now()
	task, err := container.NewTask(ctx, cio.NewCreator(cio.WithStreams(os.Stdin, iologger, iologger)))
	startVMMetric.MetricMap[metrics.NewTask] = metrics.ToUS(time.Since(tStart))
	vm.Task = &task
	if err != nil {
		return nil, nil, errors.Wrapf(err, "failed to create a task")
	}

	defer func() {
		if retErr != nil {
			if _, err := task.Delete(ctx); err != nil {
				// logger.WithError(err).Errorf("failed to delete task after failure")
				log.Info("failed to delete task after failure")
			}
		}
	}()

	// logger.Debug("StartVM: Waiting for the task to get ready")
	// log.Info("TaskWait for vmid: ", vmID)
	tStart = time.Now()
	ch, err := task.Wait(ctx)
	startVMMetric.MetricMap[metrics.TaskWait] = metrics.ToUS(time.Since(tStart))
	vm.TaskCh = ch
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to wait for a task")
	}

	defer func() {
		if retErr != nil {
			if err := task.Kill(ctx, syscall.SIGKILL); err != nil {
				// logger.WithError(err).Errorf("failed to kill task after failure")
				log.Info("failed to kill task after failure")
			}
		}
	}()
	

	// logger.Debug("StartVM: Starting the task")
	// log.Info("TaskStart for vmid: ", vmID)
	tStart = time.Now()
	if err := task.Start(ctx); err != nil {
		return nil, nil, errors.Wrap(err, "failed to start a task")
	}
	startVMMetric.MetricMap[metrics.TaskStart] = metrics.ToUS(time.Since(tStart))

	// sleep for a lil bit to see the logs
	time.Sleep(4 * time.Second)

	// kill the process and get the exit status
	// log.Info("killing task...")
	// if err := task.Kill(ctx, syscall.SIGKILL); err != nil {
	// 	log.Info("Error killing task!")
	// }

	// // wait for the process to fully exit and print out the exit status

	// // log.Info("waiting for signal")
	// status := <-ch
	// // log.Info("Getting the code...")
	// code, _, err := status.Result()
	// if err != nil {
	// 	logger.WithError(err).Errorf("failed to kill task after failure")
	// }
	// log.Info("Exited with status: ", code)
	

	defer func() {
		if retErr != nil {
			if err := task.Kill(ctx, syscall.SIGKILL); err != nil {
				logger.WithError(err).Errorf("failed to kill task after failure")
				log.Info("failed to kill task after failure")
			}
		}
	}()

	if err := os.MkdirAll(o.getVMBaseDir(vmID), 0777); err != nil {
		logger.Error("Failed to create VM base dir")
		return nil, nil, err
	}
	if o.GetUPFEnabled() {
		// log.Info("UPF enabled!!!")
		logger.Debug("Registering VM with the memory manager")

		stateCfg := manager.SnapshotStateCfg{
			VMID:             vmID,
			GuestMemPath:     o.getMemoryFile(vmID),
			BaseDir:          o.getVMBaseDir(vmID),
			GuestMemSize:     int(conf.MachineCfg.MemSizeMib) * 1024 * 1024,
			IsLazyMode:       o.isLazyMode,
			VMMStatePath:     o.getSnapshotFile(vmID),
			WorkingSetPath:   o.getWorkingSetFile(vmID),
			InstanceSockAddr: resp.UPFSockPath,
		}
		if err := o.memoryManager.RegisterVM(stateCfg); err != nil {
			return nil, nil, errors.Wrap(err, "failed to register VM with memory manager")
			// NOTE (Plamen): Potentially need a defer(DeregisteVM) here if RegisterVM is not last to execute
		}
	}

	logger.Debug("Successfully started a VM")

	// return &StartVMResponse{GuestIP: vm.Ni.PrimaryAddress}, startVMMetric, nil
	return response, startVMMetric, nil
}

func getDiskStats() (readInSector, writeInSector float64) {
	getDiskstatsCmd := "cat /proc/diskstats | grep nvme0n1"
	getDiskstatsOutput, err := exec.Command("/bin/bash", "-c", getDiskstatsCmd).Output()
	if err != nil {
		return
	}
	diskstatsStr := string(getDiskstatsOutput)
	scanner := bufio.NewScanner(strings.NewReader(diskstatsStr))
	var firstLine string
	for scanner.Scan() {
		firstLine = scanner.Text()
		// fmt.Println(firstLine)
		break
	}
	words := strings.Fields(firstLine)
	readsInSector, _ := strconv.Atoi(words[5])
	writesInSector, _ := strconv.Atoi(words[9])
	// log.Info(readsInSector)
	// log.Info(writesInSector)
	return float64(readsInSector), float64(writesInSector)
}

// StopSingleVM Shuts down a VM
// Note: VMs are not quisced before being stopped
func (o *Orchestrator) StopSingleVM(ctx context.Context, vmID string) error {
	logger := log.WithFields(log.Fields{"vmID": vmID})
	logger.Debug("Orchestrator received StopVM")

	ctx = namespaces.WithNamespace(ctx, namespaceName)
	vm, err := o.vmPool.GetVM(vmID)
	if err != nil {
		if _, ok := err.(*misc.NonExistErr); ok {
			logger.Panic("StopVM: VM does not exist")
		}
		logger.Panic("StopVM: GetVM() failed for an unknown reason")

	}

	logger = log.WithFields(log.Fields{"vmID": vmID})

	task := *vm.Task
	if err := task.Kill(ctx, syscall.SIGKILL); err != nil {
		logger.WithError(err).Error("Failed to kill the task")
		return err
	}

	<-vm.TaskCh
	//FIXME: Seems like some tasks need some extra time to die Issue#15, lr_training
	time.Sleep(500 * time.Millisecond)

	if _, err := task.Delete(ctx); err != nil {
		logger.WithError(err).Error("failed to delete task")
		return err
	}

	container := *vm.Container
	if err := container.Delete(ctx, containerd.WithSnapshotCleanup); err != nil {
		logger.WithError(err).Error("failed to delete container")
		return err
	}

	if _, err := o.fcClient.StopVM(ctx, &proto.StopVMRequest{VMID: vmID}); err != nil {
		logger.WithError(err).Error("failed to stop firecracker-containerd VM")
		return err
	}

	if err := o.vmPool.Free(vmID); err != nil {
		logger.Error("failed to free VM from VM pool")
		return err
	}

	o.workloadIo.Delete(vmID)

	logger.Debug("Stopped VM successfully")

	return nil
}

// Checks whether a URL has a .local domain
func isLocalDomain(s string) (bool, error) {
	if !strings.Contains(s, "://") {
		s = "dummy://" + s
	}

	u, err := url.Parse(s)
	if err != nil {
		return false, err
	}

	host, _, err := net.SplitHostPort(u.Host)
	if err != nil {
		host = u.Host
	}

	i := strings.LastIndex(host, ".")
	tld := host[i+1:]

	return tld == "local", nil
}

// Converts an image name to a url if it is not a URL
func getImageURL(image string) string {
	// Pull from dockerhub by default if not specified (default k8s behavior)
	if strings.Contains(image, ".") {
		return image
	}
	return "docker.io/" + image

}

func (o *Orchestrator) getImage(ctx context.Context, imageName string) (*containerd.Image, error) {
	image, found := o.cachedImages[imageName]
	if !found {
		var err error
		log.Debug(fmt.Sprintf("Pulling image %s", imageName))

		imageURL := getImageURL(imageName)
		local, _ := isLocalDomain(imageURL)
		if local {
			// Pull local image using HTTP
			resolver := docker.NewResolver(docker.ResolverOptions{
				Client: http.DefaultClient,
				Hosts: docker.ConfigureDefaultRegistries(
					docker.WithPlainHTTP(docker.MatchAllHosts),
				),
			})
			image, err = o.client.Pull(ctx, imageURL,
				containerd.WithPullUnpack,
				containerd.WithPullSnapshotter(o.snapshotter),
				containerd.WithResolver(resolver),
			)
		} else {
			// Pull remote image
			log.Info("pulling remote image: ", imageURL)
			image, err = o.client.Pull(ctx, imageURL,
				containerd.WithPullUnpack,
				containerd.WithPullSnapshotter(o.snapshotter),
			)
		}

		if err != nil {
			return &image, err
		}
		o.cachedImages[imageName] = image
	}

	return &image, nil
}

// func getK8sDNS() []string {
// 	//using googleDNS as a backup
// 	dnsIPs := []string{"8.8.8.8"}
// 	//get k8s DNS clusterIP
// 	cmd := exec.Command(
// 		"kubectl", "get", "service", "-n", "kube-system", "kube-dns", "-o=custom-columns=:.spec.clusterIP", "--no-headers",
// 	)
// 	stdoutStderr, err := cmd.CombinedOutput()
// 	if err != nil {
// 		log.Warnf("Failed to Fetch k8s dns clusterIP %v\n%s\n", err, stdoutStderr)
// 		log.Warnf("Using google dns %s\n", dnsIPs[0])
// 	} else {
// 		//adding k8s DNS clusterIP to the list
// 		dnsIPs = []string{strings.TrimSpace(string(stdoutStderr)), dnsIPs[0]}
// 	}
// 	return dnsIPs
// }

func getK8sDNS() []string {
	//using googleDNS as a backup
	dnsIPs := []string{"8.8.8.8"}
	//get k8s DNS clusterIP
	cmd := exec.Command(
		"kubectl", "get", "service", "-n", "kube-system", "kube-dns", "-o=custom-columns=:.spec.clusterIP", "--no-headers",
	)
	stdoutStderr, err := cmd.CombinedOutput()
	if err != nil {
		// log.Warnf("Failed to Fetch k8s dns clusterIP %v\n%s\n", err, stdoutStderr)
		// log.Warnf("Using google dns %s\n", dnsIPs[0])
	} else {
		//adding k8s DNS clusterIP to the list
		dnsIPs = []string{strings.TrimSpace(string(stdoutStderr)), dnsIPs[0]}
	}
	return dnsIPs
}

func (o *Orchestrator) getVMConfig(vm *misc.VM) *proto.CreateVMRequest {
	kernelArgs := "ro noapic reboot=k panic=1 pci=off nomodules systemd.log_color=false systemd.unit=firecracker.target init=/sbin/overlay-init tsc=reliable quiet 8250.nr_uarts=0 ipv6.disable=1"

	return &proto.CreateVMRequest{
		VMID:           vm.ID,
		TimeoutSeconds: 100,
		KernelArgs:     kernelArgs,
		MachineCfg: &proto.FirecrackerMachineConfiguration{
			VcpuCount:  vm.VCPUCount,
			MemSizeMib: vm.MemSizeMib,
		},
		NetworkInterfaces: []*proto.FirecrackerNetworkInterface{{
			StaticConfig: &proto.StaticNetworkConfiguration{
				MacAddress:  vm.Ni.MacAddress,
				HostDevName: vm.Ni.HostDevName,
				IPConfig: &proto.IPConfiguration{
					PrimaryAddr: vm.Ni.PrimaryAddress + vm.Ni.Subnet,
					GatewayAddr: vm.Ni.GatewayAddress,
					Nameservers: getK8sDNS(),
				},
			},
		}},
	}
}

// StopActiveVMs Shuts down all active VMs
func (o *Orchestrator) StopActiveVMs() error {
	var vmGroup sync.WaitGroup
	for vmID, vm := range o.vmPool.GetVMMap() {
		vmGroup.Add(1)
		logger := log.WithFields(log.Fields{"vmID": vmID})
		go func(vmID string, vm *misc.VM) {
			defer vmGroup.Done()
			err := o.StopSingleVM(context.Background(), vmID)
			if err != nil {
				logger.Warn(err)
			}
		}(vmID, vm)
	}

	log.Info("waiting for goroutines")
	vmGroup.Wait()
	log.Info("waiting done")

	log.Info("Closing fcClient")
	o.fcClient.Close()
	log.Info("Closing containerd client")
	o.client.Close()

	return nil
}

// PauseVM Pauses a VM
func (o *Orchestrator) PauseVM(ctx context.Context, vmID string) error {
	logger := log.WithFields(log.Fields{"vmID": vmID})
	logger.Debug("Orchestrator received PauseVM")

	ctx = namespaces.WithNamespace(ctx, namespaceName)

	if _, err := o.fcClient.PauseVM(ctx, &proto.PauseVMRequest{VMID: vmID}); err != nil {
		logger.WithError(err).Error("failed to pause the VM")
		return err
	}

	return nil
}

// ResumeVM Resumes a VM
func (o *Orchestrator) ResumeVM(ctx context.Context, vmID string) (*metrics.Metric, error) {
	var (
		resumeVMMetric *metrics.Metric = metrics.NewMetric()
		tStart         time.Time
	)

	logger := log.WithFields(log.Fields{"vmID": vmID})
	logger.Debug("Orchestrator received ResumeVM")

	ctx = namespaces.WithNamespace(ctx, namespaceName)

	tStart = time.Now()
	if _, err := o.fcClient.ResumeVM(ctx, &proto.ResumeVMRequest{VMID: vmID}); err != nil {
		logger.WithError(err).Error("failed to resume the VM")
		return nil, err
	}
	resumeVMMetric.MetricMap[metrics.FcResume] = metrics.ToUS(time.Since(tStart))

	return resumeVMMetric, nil
}

// CreateSnapshot Creates a snapshot of a VM
func (o *Orchestrator) CreateSnapshot(ctx context.Context, vmID string) error {
	logger := log.WithFields(log.Fields{"vmID": vmID})
	logger.Debug("Orchestrator received CreateSnapshot")

	ctx = namespaces.WithNamespace(ctx, namespaceName)
	clientDeadline := time.Now().Add(10000 * time.Second)
	ctxFwd, cancel := context.WithDeadline(ctx, clientDeadline)
	defer cancel()
	log.Info("CSS for: ", vmID)

	req := &proto.CreateSnapshotRequest{
		VMID:             vmID,
		SnapshotFilePath: o.getSnapshotFile(vmID),
		MemFilePath:      o.getMemoryFile(vmID),
	}

	if _, err := o.fcClient.CreateSnapshot(ctx, req); err != nil && ctxFwd.Err() == context.Canceled{
		logger.WithError(err).Error("failed to create snapshot of the VM")
		return err
	}

	return nil
}

func (o *Orchestrator) InfiniteCreateSnapshot(quit chan bool, ctx context.Context, vmID string) error {
	logger := log.WithFields(log.Fields{"vmID": vmID})
	logger.Debug("Orchestrator received CreateSnapshot")

	ctx = namespaces.WithNamespace(ctx, namespaceName)
	clientDeadline := time.Now().Add(10000 * time.Second)
	ctxFwd, cancel := context.WithDeadline(ctx, clientDeadline)
	defer cancel()

	req := &proto.CreateSnapshotRequest{
		VMID:             vmID,
		SnapshotFilePath: o.getSnapshotFile(vmID),
		MemFilePath:      o.getMemoryFile(vmID),
	}

	// log.Info("start for loop...")
	for {
		// log.Info("enter a new loop for: ", vmID)
		select {
		case <- quit:
			log.Info("quiting")
			break
		default:
			// log.Info("CSS infinitely for vmID: ", vmID)
			if _, err := o.fcClient.CreateSnapshot(ctx, req); err != nil && ctxFwd.Err() == context.Canceled{
				logger.WithError(err).Error("failed to create snapshot of the VM")
				return err
			}
			// log.Info("CSS finish for vmID: ", vmID)
		}
		//sleep? -> CSS is sleeping
	}

	return nil
}

// LoadSnapshot Loads a snapshot of a VM
func (o *Orchestrator) LoadSnapshot(ctx context.Context, vmID string) (*metrics.Metric, error) {
	var (
		loadSnapshotMetric   *metrics.Metric = metrics.NewMetric()
		tStart               time.Time
		loadErr, activateErr error
		loadDone             = make(chan int)
	)

	logger := log.WithFields(log.Fields{"vmID": vmID})
	logger.Debug("Orchestrator received LoadSnapshot")

	ctx = namespaces.WithNamespace(ctx, namespaceName)
	// log.Info("SnapshotFile: ", o.getSnapshotFile(vmID))
	// log.Info("MemoryFile: ", o.getMemoryFile(vmID))

	req := &proto.LoadSnapshotRequest{
		VMID:             vmID,
		SnapshotFilePath: o.getSnapshotFile(vmID),
		MemFilePath:      o.getMemoryFile(vmID),
		EnableUserPF:     o.GetUPFEnabled(),
	}

	if o.GetUPFEnabled() {
		log.Info("UPF enabled!!")
		if err := o.memoryManager.FetchState(vmID); err != nil {
			return nil, err
		}
	}

	tStart = time.Now()

	go func() {
		defer close(loadDone)

		if _, loadErr = o.fcClient.LoadSnapshot(ctx, req); loadErr != nil {
			logger.Error("Failed to load snapshot of the VM: ", loadErr)
		}
	}()

	if o.GetUPFEnabled() {
		if activateErr = o.memoryManager.Activate(vmID); activateErr != nil {
			logger.Warn("Failed to activate VM in the memory manager", activateErr)
		}
	}

	<-loadDone

	loadSnapshotMetric.MetricMap[metrics.LoadVMM] = metrics.ToUS(time.Since(tStart))

	if loadErr != nil || activateErr != nil {
		multierr := multierror.Of(loadErr, activateErr)
		return nil, multierr
	}

	return loadSnapshotMetric, nil
}

// Offload Shuts down the VM but leaves shim and other resources running.
func (o *Orchestrator) Offload(ctx context.Context, vmID string) error {
	logger := log.WithFields(log.Fields{"vmID": vmID})
	logger.Debug("Orchestrator received Offload")

	ctx = namespaces.WithNamespace(ctx, namespaceName)

	_, err := o.vmPool.GetVM(vmID)
	if err != nil {
		if _, ok := err.(*misc.NonExistErr); ok {
			logger.Panic("Offload: VM does not exist")
		}
		logger.Panic("Offload: GetVM() failed for an unknown reason")

	}

	if o.GetUPFEnabled() {
		if err := o.memoryManager.Deactivate(vmID); err != nil {
			logger.Error("Failed to deactivate VM in the memory manager")
			return err
		}
	}

	if _, err := o.fcClient.Offload(ctx, &proto.OffloadRequest{VMID: vmID}); err != nil {
		logger.WithError(err).Error("failed to offload the VM")
		return err
	}

	if err := o.vmPool.RecreateTap(vmID, o.hostIface); err != nil {
		logger.Error("Failed to recreate tap upon offloading")
		return err
	}

	return nil
}
