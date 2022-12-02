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
	"context"
	"encoding/csv"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"
	"flag"
	"os/exec"

	"github.com/vhive-serverless/vhive/metrics"
	ctrdlog "github.com/containerd/containerd/log"
	"github.com/containerd/containerd/namespaces"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

var (
	parallelNum = flag.Int("parallelNum", 1, "Number of parallel instances to start")
	interferNum = flag.Int("interferNum", 1, "Number of interference instances doing createsnapshot")
	// iterNum     = flag.Int("iter", 1, "Number of iterations to run")
	funcName    = flag.String("funcName", "helloworld", "Name of the function to benchmark")
	snapFilePath= flag.String("snapFilePath", "/fccd/snapshots", "path for snapshots")
	writeBW		= flag.Int("writeBW", 99999, "maximum write BW in MB/s")
	vmSize uint32 = 1024
	isVmTouch	= flag.Bool("isVmTouch", false, "preload the rootfs img and required metadata before createVM or not")
	dumpMetrics = flag.Bool("dumpMetrics", true, "dump metrics or not")
	useNVMe		= flag.Bool("useNVMe", false, "use nvme or not")
	sameCtImg = flag.Bool("sameCtImg", false, "same container image for interferon and victim or not")
	isUPFEnabled = flag.Bool("upf", false, "isUPFEnabled or not")
	isLazyMode = flag.Bool("lazy", false, "isLazyMode or not")
)
type IOWithTime struct {
    curTime time.Time
	curRead float64
    curWrite float64
}
func dropPageCache() {
	cmd := exec.Command("sudo", "/bin/sh", "-c", "sync; echo 1 > /proc/sys/vm/drop_caches")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stdout

	if err := cmd.Run(); err != nil {
		log.Fatalf("Failed to drop caches: %v", err)
	}
}

func TestSnapLoad1(t *testing.T) {
	// Need to clean up manually after this test because StopVM does not
	// work for stopping machines which are loaded from snapshots yet
	log.SetFormatter(&log.TextFormatter{
		TimestampFormat: ctrdlog.RFC3339NanoFixed,
		FullTimestamp:   true,
	})
	//log.SetReportCaller(true) // FIXME: make sure it's false unless debugging

	log.SetOutput(os.Stdout)

	log.SetLevel(log.InfoLevel)

	testTimeout := 120 * time.Second
	ctx, cancel := context.WithTimeout(namespaces.WithNamespace(context.Background(), namespaceName), testTimeout)
	defer cancel()

	orch := NewOrchestrator(
		"devmapper",
		"",
		WithTestModeOn(true),
		WithUPF(*isUPFEnabled),
		WithLazyMode(*isLazyMode),
	)

	vmID := "1"

	_, _, err := orch.StartVM(ctx, vmID, testImageName)
	require.NoError(t, err, "Failed to start VM")

	err = orch.PauseVM(ctx, vmID)
	require.NoError(t, err, "Failed to pause VM")

	err = orch.CreateSnapshot(ctx, vmID)
	require.NoError(t, err, "Failed to create snapshot of VM")
	log.Info("CSS finishes.!!")

	// _, err = orch.ResumeVM(ctx, vmID)
	// require.NoError(t, err, "Failed to resume VM")

	err = orch.Offload(ctx, vmID)
	require.NoError(t, err, "Failed to offload VM")
	time.Sleep(60*time.Second)
	dropPageCache()

	log.Info("Loading SS...")
	_, err = orch.LoadSnapshot(ctx, vmID)
	require.NoError(t, err, "Failed to load snapshot of VM")
	log.Info("resuming function...")
	_, err = orch.ResumeVM(ctx, vmID)
	require.NoError(t, err, "Failed to resume VM")
	log.Info("finish!!!")
	time.Sleep(30*time.Second)
	orch.Cleanup()
}

func TestSnapLoadMultiple(t *testing.T) {
	// Needs to be cleaned up manually.
	log.SetFormatter(&log.TextFormatter{
		TimestampFormat: ctrdlog.RFC3339NanoFixed,
		FullTimestamp:   true,
	})
	//log.SetReportCaller(true) // FIXME: make sure it's false unless debugging

	log.SetOutput(os.Stdout)

	log.SetLevel(log.InfoLevel)

	testTimeout := 120 * time.Second
	ctx, cancel := context.WithTimeout(namespaces.WithNamespace(context.Background(), namespaceName), testTimeout)
	defer cancel()

	orch := NewOrchestrator(
		"devmapper",
		"",
		WithTestModeOn(true),
		WithUPF(*isUPFEnabled),
		WithLazyMode(*isLazyMode),
	)

	vmID := "3"

	_, _, err := orch.StartVM(ctx, vmID, testImageName)
	require.NoError(t, err, "Failed to start VM")

	err = orch.PauseVM(ctx, vmID)
	require.NoError(t, err, "Failed to pause VM")

	err = orch.CreateSnapshot(ctx, vmID)
	require.NoError(t, err, "Failed to create snapshot of VM")

	err = orch.Offload(ctx, vmID)
	require.NoError(t, err, "Failed to offload VM")

	_, err = orch.LoadSnapshot(ctx, vmID)
	require.NoError(t, err, "Failed to load snapshot of VM")

	_, err = orch.ResumeVM(ctx, vmID)
	require.NoError(t, err, "Failed to resume VM")

	err = orch.Offload(ctx, vmID)
	require.NoError(t, err, "Failed to offload VM")

	_, err = orch.LoadSnapshot(ctx, vmID)
	require.NoError(t, err, "Failed to load snapshot of VM")

	_, err = orch.ResumeVM(ctx, vmID)
	require.NoError(t, err, "Failed to resume VM, ")

	err = orch.Offload(ctx, vmID)
	require.NoError(t, err, "Failed to offload VM")

	orch.Cleanup()
}

func TestParallelSnapLoad(t *testing.T) {
	// Needs to be cleaned up manually.
	log.SetFormatter(&log.TextFormatter{
		TimestampFormat: ctrdlog.RFC3339NanoFixed,
		FullTimestamp:   true,
	})
	//log.SetReportCaller(true) // FIXME: make sure it's false unless debugging

	log.SetOutput(os.Stdout)

	log.SetLevel(log.InfoLevel)

	testTimeout := 120 * time.Second
	ctx, cancel := context.WithTimeout(namespaces.WithNamespace(context.Background(), namespaceName), testTimeout)
	defer cancel()

	vmNum := 5
	vmIDBase := 6

	orch := NewOrchestrator(
		"devmapper",
		"",
		WithTestModeOn(true),
		WithUPF(*isUPFEnabled),
		WithLazyMode(*isLazyMode),
	)

	// Pull image
	_, err := orch.getImage(ctx, testImageName)
	require.NoError(t, err, "Failed to pull image "+testImageName)

	var vmGroup sync.WaitGroup
	for i := 0; i < vmNum; i++ {
		vmGroup.Add(1)
		go func(i int) {
			defer vmGroup.Done()
			vmID := fmt.Sprintf("%d", i+vmIDBase)

			_, _, err := orch.StartVM(ctx, vmID, testImageName)
			require.NoError(t, err, "Failed to start VM, "+vmID)

			err = orch.PauseVM(ctx, vmID)
			require.NoError(t, err, "Failed to pause VM, "+vmID)

			err = orch.CreateSnapshot(ctx, vmID)
			require.NoError(t, err, "Failed to create snapshot of VM, "+vmID)

			err = orch.Offload(ctx, vmID)
			require.NoError(t, err, "Failed to offload VM, "+vmID)

			_, err = orch.LoadSnapshot(ctx, vmID)
			require.NoError(t, err, "Failed to load snapshot of VM, "+vmID)

			_, err = orch.ResumeVM(ctx, vmID)
			require.NoError(t, err, "Failed to resume VM, "+vmID)
		}(i)
	}
	vmGroup.Wait()

	orch.Cleanup()
}

func TestParallelPhasedSnapLoad(t *testing.T) {
	// Needs to be cleaned up manually.
	log.SetFormatter(&log.TextFormatter{
		TimestampFormat: ctrdlog.RFC3339NanoFixed,
		FullTimestamp:   true,
	})
	//log.SetReportCaller(true) // FIXME: make sure it's false unless debugging

	log.SetOutput(os.Stdout)

	log.SetLevel(log.InfoLevel)

	testTimeout := 120 * time.Second
	ctx, cancel := context.WithTimeout(namespaces.WithNamespace(context.Background(), namespaceName), testTimeout)
	defer cancel()

	vmNum := 10
	vmIDBase := 11

	orch := NewOrchestrator(
		"devmapper",
		"",
		WithTestModeOn(true),
		WithUPF(*isUPFEnabled),
		WithLazyMode(*isLazyMode),
	)

	// Pull image
	_, err := orch.getImage(ctx, testImageName)
	require.NoError(t, err, "Failed to pull image "+testImageName)

	{
		var vmGroup sync.WaitGroup
		for i := 0; i < vmNum; i++ {
			vmGroup.Add(1)
			go func(i int) {
				defer vmGroup.Done()
				vmID := fmt.Sprintf("%d", i+vmIDBase)
				_, _, err := orch.StartVM(ctx, vmID, testImageName)
				require.NoError(t, err, "Failed to start VM, "+vmID)
			}(i)
		}
		vmGroup.Wait()
	}

	{
		var vmGroup sync.WaitGroup
		for i := 0; i < vmNum; i++ {
			vmGroup.Add(1)
			go func(i int) {
				defer vmGroup.Done()
				vmID := fmt.Sprintf("%d", i+vmIDBase)
				err := orch.PauseVM(ctx, vmID)
				require.NoError(t, err, "Failed to pause VM, "+vmID)
			}(i)
		}
		vmGroup.Wait()
	}

	{
		var vmGroup sync.WaitGroup
		for i := 0; i < vmNum; i++ {
			vmGroup.Add(1)
			go func(i int) {
				defer vmGroup.Done()
				vmID := fmt.Sprintf("%d", i+vmIDBase)
				err := orch.CreateSnapshot(ctx, vmID)
				require.NoError(t, err, "Failed to create snapshot of VM, "+vmID)
			}(i)
		}
		vmGroup.Wait()
	}

	{
		var vmGroup sync.WaitGroup
		for i := 0; i < vmNum; i++ {
			vmGroup.Add(1)
			go func(i int) {
				defer vmGroup.Done()
				vmID := fmt.Sprintf("%d", i+vmIDBase)
				err := orch.Offload(ctx, vmID)
				require.NoError(t, err, "Failed to offload VM, "+vmID)
			}(i)
		}
		vmGroup.Wait()
	}

	{
		var vmGroup sync.WaitGroup
		for i := 0; i < vmNum; i++ {
			vmGroup.Add(1)
			go func(i int) {
				defer vmGroup.Done()
				vmID := fmt.Sprintf("%d", i+vmIDBase)
				_, err := orch.LoadSnapshot(ctx, vmID)
				require.NoError(t, err, "Failed to load snapshot of VM, "+vmID)
			}(i)
		}
		vmGroup.Wait()
	}

	{
		var vmGroup sync.WaitGroup
		for i := 0; i < vmNum; i++ {
			vmGroup.Add(1)
			go func(i int) {
				defer vmGroup.Done()
				vmID := fmt.Sprintf("%d", i+vmIDBase)
				_, err := orch.ResumeVM(ctx, vmID)
				require.NoError(t, err, "Failed to resume VM, "+vmID)
			}(i)
		}
		vmGroup.Wait()
	}

	orch.Cleanup()
}

func TestSequentialCSS(t *testing.T) {
	var (
		serveMetrics = make([]*metrics.Metric, *parallelNum)//intf:8 parallelNum:1
		
	)
	for i := 0; i < *parallelNum; i++ {
		serveMetrics[i] = metrics.NewMetric()
	}

	log.SetFormatter(&log.TextFormatter{
		TimestampFormat: ctrdlog.RFC3339NanoFixed,
		FullTimestamp:   true,
	})

	log.SetOutput(os.Stdout)

	log.SetLevel(log.InfoLevel)

	testTimeout := 300 * time.Second
	ctx, cancel := context.WithTimeout(namespaces.WithNamespace(context.Background(), namespaceName), testTimeout)
	defer cancel()

	vmNum := *parallelNum
	vmIDBase := 0

	// orch := NewOrchestrator(
	// 	"devmapper",
	// 	"",
	// 	"",
	// 	"",
	// 	10,
	// 	WithTestModeOn(true),
	// 	WithUPF(*isUPFEnabled),
	// 	WithLazyMode(*isLazyMode),
	// 	WithFullLocal(*isFullLocal),
	// 	WithMetricsMode(true),
	// 	WithSnapshotsDir(*snapFilePath),
	// )
	orch := NewOrchestrator(
		"devmapper",
		"",
		WithTestModeOn(true),
		WithUPF(false),
		WithLazyMode(*isLazyMode),
	)
	defer orch.Cleanup()

	// Pull image
	log.Info("pulling intf image now......")
	_, err := orch.getImage(ctx, testImageName)
	require.NoError(t, err, "Failed to pull image "+testImageName)
	ImageName := testImageName
	if !*sameCtImg {
		ImageName = testImageNamePyaes
		log.Info("pulling victim image now......")
		_, err := orch.getImage(ctx, testImageNamePyaes)
		require.NoError(t, err, "Failed to pull image "+testImageNamePyaes)
	}
	
	// log.Info("pull complete, starting Victim VM ...")

	var dummyInterferon int
	log.Info("Starting Intf VM ...")
	
		var vmGroup sync.WaitGroup
		if *interferNum == 0 {
			dummyInterferon = 1
		} else {
			dummyInterferon = *interferNum
		}
		var CreateSSInstancePid = make([]string, dummyInterferon)
		for i := 0; i < dummyInterferon; i++ {
			vmGroup.Add(1)
			go func(i int) {
				defer vmGroup.Done()
				vmID := fmt.Sprintf("%d", i+vmIDBase)
				// var tStart = time.Now()
				response, _, err := orch.StartVMModified(ctx, vmID, testImageName, vmSize, 1)
				log.Info("CSS FcPid: ", response.FCPid)
				CreateSSInstancePid[i] = response.FCPid
				// serveMetrics[i].MetricMap[metrics.StartVM] = metrics.ToUS(time.Since(tStart))
				// if metr != nil {
				// 	for k, v := range metr.MetricMap {
				// 		serveMetrics[i].MetricMap[k] = v
				// 	}
				// }
				require.NoError(t, err, "Failed to start VM, "+vmID)
			}(i)
		}
		vmGroup.Wait()
	

	log.Info("Pausing Intf VM ...")
	{
		var vmGroup sync.WaitGroup
		for i := 0; i < dummyInterferon; i++ {
			vmGroup.Add(1)
			go func(i int) {
				defer vmGroup.Done()
				vmID := fmt.Sprintf("%d", i+vmIDBase)
				// var tStart = time.Now()
				err := orch.PauseVM(ctx, vmID)
				// serveMetrics[i].MetricMap[metrics.PauseVM] = metrics.ToUS(time.Since(tStart))
				require.NoError(t, err, "Failed to pause VM, "+vmID)
			}(i)
		}
		vmGroup.Wait()
	}

	// throttle interferon writeBW
	if (*writeBW == 99999) {
		log.Info("resetting to no throttling...")
		throttleIoMaxCmd := "echo \"259:0 wbps=max\" | sudo tee /sys/fs/cgroup/test/io.max"
		exec.Command("/bin/bash", "-c", throttleIoMaxCmd).Start()
	} else{
		// echo max BW into io.max
		maxWriteBWByte := 1024*1024*(*writeBW)
		log.Info("throttling to ", maxWriteBWByte)
		throttleIoMaxCmd := fmt.Sprintf("echo \"259:0 wbps=%d\" | sudo tee /sys/fs/cgroup/test/io.max", maxWriteBWByte)
		exec.Command("/bin/bash", "-c", throttleIoMaxCmd).Start()
		// put createss pid(s) into procs 
		for i := 0; i < *interferNum; i++ {
			throttlePidCmd := fmt.Sprintf("echo %s | sudo tee /sys/fs/cgroup/test/cgroup.procs", CreateSSInstancePid[i])
			throttlePidCmdExec := exec.Command("/bin/bash", "-c", throttlePidCmd)
			stdout, err := throttlePidCmdExec.Output()
			if err != nil {
				fmt.Println(err.Error())
				return
			}
			fmt.Println(string(stdout))
		}
	}

	log.Info("Creating Seq Intf Snapshots ...")
	// quit := make(chan bool)
	// for i := 0; i < *interferNum; i++ {
	// 	go func(i int) {
	// 		vmID := fmt.Sprintf("%d", i+vmIDBase)
	// 		err := orch.InfiniteCreateSnapshot(quit, ctx, vmID)
	// 		require.NoError(t, err, "Failed to create snapshot of VM, "+vmID)
	// 	}(i)
	// }
	// readInSectorBeforeRun, writeInSectorBeforeRun := getDiskStats()

	var intfGroup sync.WaitGroup
	if *interferNum != 0  {
		for i := 0; i < *interferNum; i++ {
			intfGroup.Add(1)
			go func(i int) {
				defer intfGroup.Done()
				vmID := fmt.Sprintf("%d", i+vmIDBase)
				err := orch.CreateSnapshot(ctx, vmID)
				// // serveMetrics[i].MetricMap[metrics.CreateSnapshot] = metrics.ToUS(time.Since(tStart))
				require.NoError(t, err, "Failed to create snapshot of VM, "+vmID)
				log.Info("CSS finish!!!")
			}(i)
		}
	}

	// wait for a few second to make sure intf doing disk write
	// time.Sleep(4*time.Second)
	
	
	// start victim
	log.Info("start victim VMs...")
	{
		var victimGroup sync.WaitGroup
		for i := dummyInterferon; i < dummyInterferon+vmNum; i++ {
			victimGroup.Add(1)
			go func(i, x int) {
				defer victimGroup.Done()
				victimID := fmt.Sprintf("%d", i+vmIDBase)
				
				// var tStart = time.Now()
				response, metr, err := orch.StartVMModified(ctx, victimID, ImageName, 256, 1)
				// serveMetrics[x].MetricMap[metrics.StartVM] = metrics.ToUS(time.Since(tStart))
				log.Info("Victim FCPid: ", response.FCPid)
				if metr != nil {
					for k, v := range metr.MetricMap {
						serveMetrics[x].MetricMap[k] = v
					}
				}
				require.NoError(t, err, "Failed to start VM, "+victimID)
			}(i, i-dummyInterferon)
		}
		victimGroup.Wait()
	}

	log.Info("Start VM Finishes here ...")
	intfGroup.Wait()
	// if *interferNum != 0 {
	// 	for i := 0; i < *interferNum; i++ {
	// 		quit <- true
	// 	}
	// }
	log.Info("All Create Snapshot threads have finished or exited ...")
	// readInSectorAfterRun, writeInSectorAfterRun := getDiskStats()
	// log.Info("Read duing CSS in MB: ", (readInSectorAfterRun - readInSectorBeforeRun)*512/1024/1024)
	// log.Info("Write duing CSS in MB: ", (writeInSectorAfterRun - writeInSectorBeforeRun)*512/1024/1024)
	// time.Sleep(5*time.Second)//wait for function to finish
	

	if *dumpMetrics {
		var upfMetrics = make([]*metrics.Metric, *parallelNum)
		// var diff_or_same = "diff"
		// if *sameCtImg {
		// 	diff_or_same = "same"
		// }
		filePath := fmt.Sprintf("./verify/%d_%d_%d.csv" , *parallelNum, *interferNum, *writeBW)
		// if !*sameCtImg {
		// 	filePath = fmt.Sprintf("./test_diffCSSNC/%d_%d_%d.csv" , *parallelNum, *interferNum, *writeBW)
		// }
		// vmtouchExt := ""
		// if *isVmTouch {
		// 	vmtouchExt = "_vmtouch"
		// }
		// filePath := fmt.Sprintf("./Seq_CreateSS_%d_%d_throttle/Seq_CreateSS_%d%s.csv" , *parallelNum, *interferNum, *writeBW, vmtouchExt)
		notUsingUpf := false

		fusePrintMetrics(t, serveMetrics, upfMetrics, &notUsingUpf, true, *funcName, filePath)
	}

	// killpidstatCmd := "/home/cc/vHive/ctriface/kill_running_tools.sh"
	// exec.Command("/bin/bash", "-c", killpidstatCmd).Start()
}

func TestOnlyCSS(t *testing.T) {
	
	var (
		// serveMetrics = make([]*metrics.Metric, *interferNum)//intf:8 parallelNum:1
		CreateSSInstancePid = make([]string, *interferNum)
	)
	for i := 0; i < *parallelNum; i++ {
		// serveMetrics[i] = metrics.NewMetric()
	}

	log.SetFormatter(&log.TextFormatter{
		TimestampFormat: ctrdlog.RFC3339NanoFixed,
		FullTimestamp:   true,
	})

	log.SetOutput(os.Stdout)

	log.SetLevel(log.InfoLevel)

	testTimeout := 120 * time.Second
	ctx, cancel := context.WithTimeout(namespaces.WithNamespace(context.Background(), namespaceName), testTimeout)
	defer cancel()

	// vmNum := *parallelNum
	vmIDBase := 0

	// orch := NewOrchestrator(
	// 	"devmapper",
	// 	"",
	// 	"",
	// 	"",
	// 	10,
	// 	WithTestModeOn(true),
	// 	WithUPF(*isUPFEnabled),
	// 	WithLazyMode(*isLazyMode),
	// 	WithFullLocal(*isFullLocal),
	// 	WithMetricsMode(true),
	// 	WithSnapshotsDir(*snapFilePath),
	// )
	orch := NewOrchestrator(
		"devmapper",
		"",
		WithTestModeOn(true),
		WithUPF(*isUPFEnabled),
		WithLazyMode(*isLazyMode),
	)
	defer orch.Cleanup()

	// Pull image
	_, err := orch.getImage(ctx, testImageName)
	require.NoError(t, err, "Failed to pull image "+testImageName)

	log.Info("Starting Intf VM ...")
	{
		var vmGroup sync.WaitGroup
		for i := 0; i < *interferNum; i++ {
			vmGroup.Add(1)
			go func(i int) {
				defer vmGroup.Done()
				vmID := fmt.Sprintf("%d", i+vmIDBase)
				// var tStart = time.Now()
				response, _, err := orch.StartVMModified(ctx, vmID, testImageName, vmSize, 1)
				log.Info("CSS FcPid: ", response.FCPid)
				CreateSSInstancePid[i] = response.FCPid
				// serveMetrics[i].MetricMap[metrics.StartVM] = metrics.ToUS(time.Since(tStart))
				// if metr != nil {
				// 	for k, v := range metr.MetricMap {
				// 		serveMetrics[i].MetricMap[k] = v
				// 	}
				// }
				require.NoError(t, err, "Failed to start VM, "+vmID)
			}(i)
		}
		vmGroup.Wait()
	}

	log.Info("Pausing Intf VM ...")
	{
		var vmGroup sync.WaitGroup
		for i := 0; i < *interferNum; i++ {
			vmGroup.Add(1)
			go func(i int) {
				defer vmGroup.Done()
				vmID := fmt.Sprintf("%d", i+vmIDBase)
				// var tStart = time.Now()
				err := orch.PauseVM(ctx, vmID)
				// serveMetrics[i].MetricMap[metrics.PauseVM] = metrics.ToUS(time.Since(tStart))
				require.NoError(t, err, "Failed to pause VM, "+vmID)
			}(i)
		}
		vmGroup.Wait()
	}

	// throttle interferon writeBW
	if (*writeBW == 99999) {
		log.Info("resetting to no throttling...")
		throttleIoMaxCmd := "echo \"259:0 wbps=max\" | sudo tee /sys/fs/cgroup/test/io.max"
		exec.Command("/bin/bash", "-c", throttleIoMaxCmd).Start()
	} else{
		// echo max BW into io.max
		maxWriteBWByte := 1024*1024*(*writeBW)
		log.Info("throttling to ", maxWriteBWByte)
		throttleIoMaxCmd := fmt.Sprintf("echo \"259:0 wbps=%d\" | sudo tee /sys/fs/cgroup/test/io.max", maxWriteBWByte)
		exec.Command("/bin/bash", "-c", throttleIoMaxCmd).Start()
		// put createss pid(s) into procs 
		for i := 0; i < *interferNum; i++ {
			throttlePidCmd := fmt.Sprintf("echo %s | sudo tee /sys/fs/cgroup/test/cgroup.procs", CreateSSInstancePid[i])
			throttlePidCmdExec := exec.Command("/bin/bash", "-c", throttlePidCmd)
			stdout, err := throttlePidCmdExec.Output()
			if err != nil {
				fmt.Println(err.Error())
				return
			}
			fmt.Println(string(stdout))
		}
	}
	
	readInSectorBeforeRun, writeInSectorBeforeRun := getDiskStats()

	var curReadSectors, curWriteSectors, prevReadSectors, prevWriteSectors, writesPerSecond, readsPerSecond float64
	var allRecords [] IOWithTime
    var curRecord IOWithTime
	var getIOTimeGroup sync.WaitGroup
	getIOTimeGroup.Add(1)
	go func() {
		defer getIOTimeGroup.Done()
		for i := 0; i < 30; i++{
			curReadSectors, curWriteSectors = getDiskStats()
			if i == 0 {
				readsPerSecond = (curReadSectors - readInSectorBeforeRun)*512/1024/1024
				writesPerSecond = (curWriteSectors - writeInSectorBeforeRun)*512/1024/1024
			}else {
				readsPerSecond = (curReadSectors - prevReadSectors)*512/1024/1024
				writesPerSecond = (curWriteSectors - prevWriteSectors)*512/1024/1024
			}
			log.Info("Read duing CSS in MB: ", readsPerSecond)
			log.Info("Write duing CSS in MB: ", writesPerSecond)
			curRecord = IOWithTime{time.Now(), readsPerSecond, writesPerSecond}
			allRecords = append(allRecords, curRecord)
			// update prevSectors
			prevReadSectors = curReadSectors
			prevWriteSectors = curWriteSectors
			time.Sleep(1 * time.Second)	
		}
	}()

	log.Info("Creating Intf Snapshots ...")
	var intfGroup sync.WaitGroup
	if *interferNum != 0 {
		for i := 0; i < *interferNum; i++ {
			intfGroup.Add(1)
			// time.Sleep(3*time.Second)
			go func(i int) {
				defer intfGroup.Done()
				log.Info("creating SS for: ", i)
				vmID := fmt.Sprintf("%d", i)
				err :=orch.CreateSnapshot(ctx, vmID)
				// serveMetrics[i].MetricMap[metrics.CreateSnapshot] = metrics.ToUS(time.Since(tStart))
				require.NoError(t, err, "Failed to create snapshot of VM, "+vmID)
				log.Info("CSS finish for vmID: ", vmID)
			}(i)
		}
	}
	
	log.Info("All Create Snapshot threads have finished or exited. Syncing...")
	syncCmd := "sync"
	exec.Command("sudo", "/bin/bash", "-c", syncCmd).Start()
	// log.Info("sleep for a lil to wait for flushing")

	getIOTimeGroup.Wait()
	TestwriteDiskIoWithTime(allRecords)
	

	// if *dumpMetrics {
	// 	vmtouchExt := ""
	// 	if *isVmTouch {
	// 		vmtouchExt = "_vmtouch"
	// 	}
	// 	var upfMetrics = make([]*metrics.Metric, *parallelNum)

	// 	filePath := fmt.Sprintf("./Seq_CreateSS_%d_%d/Seq_CreateSS_%d%s.csv" , *parallelNum, *interferNum, *writeBW, vmtouchExt)
	// 	notUsingUpf := false

	// 	fusePrintMetrics(t, serveMetrics, upfMetrics, &notUsingUpf, true, *funcName, filePath)
	// }
	
}

func fusePrintMetrics(t *testing.T, serveMetrics, upfMetrics []*metrics.Metric, isUPFEnabled *bool, printIndiv bool, funcName, outfile string) {
	outFileName := outfile

	if *isUPFEnabled {
		for i, metr := range serveMetrics {
			for k, v := range upfMetrics[i].MetricMap {
				metr.MetricMap[k] = v
			}
		}
		// log.Info("congrats! upf enabled!!")
	}

	if printIndiv {
		for _, metr := range serveMetrics {
			err := metrics.PrintMeanStd(outFileName, funcName, metr)
			require.NoError(t, err, "Failed to dump stats")
		}
	}

	err := metrics.PrintMeanStd(outFileName, funcName, serveMetrics...)
	require.NoError(t, err, "Failed to dump stats")
}

func TestwriteDiskIoWithTime(records []IOWithTime) {
	// records := []IOWithTime{
    //     {time.Now(), 4, 7},
    //     {time.Now(), 25, 0},
    //     {time.Now(), 25, 0},
    // }
    file, err := os.Create("record.csv")
    defer file.Close()
    if err != nil {
        log.Fatalln("failed to open file", err)
    }
    w := csv.NewWriter(file)
    defer w.Flush()
	// write header
    row := []string{"Timeline", "Read(MB)/s", "Write(MB)/s"}
    if err := w.Write(row); err != nil {
        log.Fatalln("error writing record to file", err)
    }
    // Using Write
    for _, record := range records {
		var curTimeFormatted = fmt.Sprintf("%d:%d:%d", record.curTime.Hour(), record.curTime.Minute(), record.curTime.Second())
        row := []string{curTimeFormatted, fmt.Sprintf("%f", record.curRead), fmt.Sprintf("%f", record.curWrite)}
        if err := w.Write(row); err != nil {
            log.Fatalln("error writing record to file", err)
        }
    }
    
    // Using WriteAll
    // var data [][]string
    // for _, record := range records {
    //     row := []string{record.curTime.String(), fmt.Sprintf("%f", record.curRead), fmt.Sprintf("%f", record.curRead)}
    //     data = append(data, row)
    // }
    // w.WriteAll(data)
}

func TestLoadSnap(t *testing.T) {
	var (
		serveMetrics = make([]*metrics.Metric, *parallelNum)
		
	)
	for i := 0; i < *parallelNum; i++ {
		serveMetrics[i] = metrics.NewMetric()
	}

	log.SetFormatter(&log.TextFormatter{
		TimestampFormat: ctrdlog.RFC3339NanoFixed,
		FullTimestamp:   true,
	})

	log.SetOutput(os.Stdout)

	log.SetLevel(log.InfoLevel)

	testTimeout := 300 * time.Second
	ctx, cancel := context.WithTimeout(namespaces.WithNamespace(context.Background(), namespaceName), testTimeout)
	defer cancel()

	vmNum := *parallelNum
	// vmIDBase := 0

	orch := NewOrchestrator(
		"devmapper",
		"",
		WithTestModeOn(true),
		WithUPF(false),
		WithLazyMode(*isLazyMode),
	)
	defer orch.Cleanup()

	// Pull image
	log.Info("pulling intf image now......")
	_, err := orch.getImage(ctx, testImageName)
	require.NoError(t, err, "Failed to pull image "+testImageName)
	ImageName := testImageName
	if !*sameCtImg {
		ImageName = testImageNamePyaes
		log.Info("pulling victim image now......")
		_, err := orch.getImage(ctx, testImageNamePyaes)
		require.NoError(t, err, "Failed to pull image "+testImageNamePyaes)
	}

	var vmGroup sync.WaitGroup
	var dummyInterferon int
	if *interferNum == 0 {
		dummyInterferon = 1
	} else {
		dummyInterferon = *interferNum
	}
	var CreateSSInstancePid = make([]string, dummyInterferon)
	log.Info("Starting Intf VM ...")
	for i := 0; i < dummyInterferon; i++ {
		vmGroup.Add(1)
		go func(i int) {
			defer vmGroup.Done()
			vmID := fmt.Sprintf("%d", i)
			response, _, err := orch.StartVMModified(ctx, vmID, testImageName, vmSize, 1)
			log.Info("CSS FcPid: ", response.FCPid)
			CreateSSInstancePid[i] = response.FCPid
			require.NoError(t, err, "Failed to start VM, "+vmID)
		}(i)
	}
	vmGroup.Wait()

	log.Info("Pausing Intf VM ...")
	{
		var vmGroup sync.WaitGroup
		for i := 0; i < dummyInterferon; i++ {
			vmGroup.Add(1)
			go func(i int) {
				defer vmGroup.Done()
				vmID := fmt.Sprintf("%d", i)
				// var tStart = time.Now()
				err := orch.PauseVM(ctx, vmID)
				// serveMetrics[i].MetricMap[metrics.PauseVM] = metrics.ToUS(time.Since(tStart))
				require.NoError(t, err, "Failed to pause VM, "+vmID)
			}(i)
		}
		vmGroup.Wait()
	}

	// throttle interferon writeBW
	if (*writeBW == 99999) {
		log.Info("resetting to no throttling...")
		throttleIoMaxCmd := "echo \"259:0 wbps=max\" | sudo tee /sys/fs/cgroup/test/io.max"
		exec.Command("/bin/bash", "-c", throttleIoMaxCmd).Start()
	} else{
		// echo max BW into io.max
		maxWriteBWByte := 1024*1024*(*writeBW)
		log.Info("throttling to ", maxWriteBWByte)
		throttleIoMaxCmd := fmt.Sprintf("echo \"259:0 wbps=%d\" | sudo tee /sys/fs/cgroup/test/io.max", maxWriteBWByte)
		exec.Command("/bin/bash", "-c", throttleIoMaxCmd).Start()
		// put createss pid(s) into procs 
		for i := 0; i < *interferNum; i++ {
			throttlePidCmd := fmt.Sprintf("echo %s | sudo tee /sys/fs/cgroup/test/cgroup.procs", CreateSSInstancePid[i])
			throttlePidCmdExec := exec.Command("/bin/bash", "-c", throttlePidCmd)
			stdout, err := throttlePidCmdExec.Output()
			if err != nil {
				fmt.Println(err.Error())
				return
			}
			fmt.Println(string(stdout))
		}
	}

	log.Info("Starting victim VMs ...")
	{
		var victimGroup sync.WaitGroup
		for i := dummyInterferon; i < dummyInterferon+vmNum; i++ {
			victimGroup.Add(1)
			go func(i, x int) {
				defer victimGroup.Done()
				victimID := fmt.Sprintf("%d", i)
				response, metr, err := orch.StartVMModified(ctx, victimID, ImageName, 256, 1)
				// serveMetrics[x].MetricMap[metrics.StartVM] = metrics.ToUS(time.Since(tStart))
				log.Info("Victim FCPid: ", response.FCPid)
				if metr != nil {
					for k, v := range metr.MetricMap {
						serveMetrics[x].MetricMap[k] = v
					}
				}
				require.NoError(t, err, "Failed to start VM, "+victimID)
			}(i, i-dummyInterferon)
		}
		victimGroup.Wait()
	}
	// if *interferNum != 0 {
	// 	for i := 0; i < *interferNum; i++ {
	// 		quit <- true
	// 	}
	// }
	log.Info("Pausing victim VM ...")
	{
		var vmGroup sync.WaitGroup
		for i := dummyInterferon; i < dummyInterferon+vmNum; i++ {
			vmGroup.Add(1)
			go func(i int) {
				defer vmGroup.Done()
				vmID := fmt.Sprintf("%d", i)
				// var tStart = time.Now()
				err := orch.PauseVM(ctx, vmID)
				// serveMetrics[i].MetricMap[metrics.PauseVM] = metrics.ToUS(time.Since(tStart))
				require.NoError(t, err, "Failed to pause VM, "+vmID)
			}(i)
		}
		vmGroup.Wait()
	}
	// readInSectorAfterRun, writeInSectorAfterRun := getDiskStats()
	// log.Info("Read duing CSS in MB: ", (readInSectorAfterRun - readInSectorBeforeRun)*512/1024/1024)
	// log.Info("Write duing CSS in MB: ", (writeInSectorAfterRun - writeInSectorBeforeRun)*512/1024/1024)
	// time.Sleep(5*time.Second)//wait for function to finish
	log.Info("CSS VM ...")
	{
		var intfGroup sync.WaitGroup
		for i := dummyInterferon; i < dummyInterferon+vmNum; i++ {
			intfGroup.Add(1)
			go func(i int) {
				defer intfGroup.Done()
				vmID := fmt.Sprintf("%d", i)
				err := orch.CreateSnapshot(ctx, vmID)
				require.NoError(t, err, "Failed to create snapshot of VM, "+vmID)
			}(i)
		}
		intfGroup.Wait()
	}
	log.Info("Syncing victim Snapshots...")
	exec.Command("sudo", "/bin/bash", "-c", "sync").Start()
			
	log.Info("offloading victim ...")
	{
		var vmGroup sync.WaitGroup
		for i := dummyInterferon; i < dummyInterferon+vmNum; i++ {
			vmGroup.Add(1)
			go func(i int) {
				defer vmGroup.Done()
				vmID := fmt.Sprintf("%d", i)
				err := orch.Offload(ctx, vmID)
				require.NoError(t, err, "Failed to offload VM, "+vmID)
			}(i)
		}
		vmGroup.Wait()
	}

	log.Info("Creating Intf Snapshots... in BG")
	if *interferNum != 0  {
		var intfGroup sync.WaitGroup
		for i := 0; i < *interferNum; i++ {
			intfGroup.Add(1)
			go func(i int) {
				defer intfGroup.Done()
				// log.Info("creating SS for: ", i)
				vmID := fmt.Sprintf("%d", i)
				err := orch.CreateSnapshot(ctx, vmID)
				require.NoError(t, err, "Failed to create snapshot of VM, "+vmID)
			}(i)
		}
		intfGroup.Wait()
	}
	go func() {
		log.Info("All Create Snapshot threads have finished or exited. Syncing...")
		exec.Command("sudo", "/bin/bash", "-c", "sync").Start()
	}()

	log.Info("victim loading SS ...")
	{
		var vmGroup sync.WaitGroup
		for i := dummyInterferon; i < dummyInterferon+vmNum; i++ {
			vmGroup.Add(1)
			go func(i int) {
				defer vmGroup.Done()
				vmID := fmt.Sprintf("%d", i)
				_, err := orch.LoadSnapshot(ctx, vmID)
				require.NoError(t, err, "Failed to load snapshot of VM, "+vmID)
			}(i)
		}
		vmGroup.Wait()
	}

	log.Info("victim resuming SS ...")
	{
		var vmGroup sync.WaitGroup
		for i := dummyInterferon; i < dummyInterferon+vmNum; i++ {
			vmGroup.Add(1)
			go func(i int) {
				defer vmGroup.Done()
				vmID := fmt.Sprintf("%d", i)
				_, err := orch.ResumeVM(ctx, vmID)
				require.NoError(t, err, "Failed to resume VM, "+vmID)
			}(i)
		}
		vmGroup.Wait()
	}
	
	// time.Sleep(9999*time.Second)

	// _, err = orch.LoadSnapshot(ctx, vmID)
	// require.NoError(t, err, "Failed to load snapshot of VM")

	// _, err = orch.ResumeVM(ctx, vmID)
	// require.NoError(t, err, "Failed to resume VM")


	// if *dumpMetrics {
	// 	var upfMetrics = make([]*metrics.Metric, *parallelNum)
	// 	// var diff_or_same = "diff"
	// 	// if *sameCtImg {
	// 	// 	diff_or_same = "same"
	// 	// }
	// 	filePath := fmt.Sprintf("./verify/%d_%d_%d.csv" , *parallelNum, *interferNum, *writeBW)
	// 	// if !*sameCtImg {
	// 	// 	filePath = fmt.Sprintf("./test_diffCSSNC/%d_%d_%d.csv" , *parallelNum, *interferNum, *writeBW)
	// 	// }
	// 	// vmtouchExt := ""
	// 	// if *isVmTouch {
	// 	// 	vmtouchExt = "_vmtouch"
	// 	// }
	// 	// filePath := fmt.Sprintf("./Seq_CreateSS_%d_%d_throttle/Seq_CreateSS_%d%s.csv" , *parallelNum, *interferNum, *writeBW, vmtouchExt)
	// 	notUsingUpf := false

	// 	fusePrintMetrics(t, serveMetrics, upfMetrics, &notUsingUpf, true, *funcName, filePath)
	// }
}