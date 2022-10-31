# MIT License
#
# Copyright (c) 2020 Dmitrii Ustiugov, Plamen Petrov and EASE lab
#
# Permission is hereby granted, free of charge, to any person obtaining a copy
# of this software and associated documentation files (the "Software"), to deal
# in the Software without restriction, including without limitation the rights
# to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
# copies of the Software, and to permit persons to whom the Software is
# furnished to do so, subject to the following conditions:
#
# The above copyright notice and this permission notice shall be included in all
# copies or substantial portions of the Software.
#
# THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
# IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
# FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
# AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
# LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
# OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
# SOFTWARE.

EXTRAGOARGS:=-v -race -cover
EXTRATESTFILES:=iface_test.go iface.go orch_options.go orch.go types.go
BENCHFILES:=bench_test.go iface.go orch_options.go orch.go
WITHUPF:=-upf
WITHLAZY:=-lazy
GOBENCH:=-v -timeout 1500s
CTRDLOGDIR:=/tmp/ctrd-logs

writeBW:=99999
isVmTouch:=false
parallelNum:=1
interferNum:=1
dumpMetrics:=true
useNVMe:=false
sameCtImg:=false

test:
	./../scripts/clean_fcctr.sh
	sudo mkdir -m777 -p $(CTRDLOGDIR) && sudo env "PATH=$(PATH)" /usr/local/bin/firecracker-containerd --config /etc/firecracker-containerd/config.toml 1>$(CTRDLOGDIR)/ctriface_log.out 2>$(CTRDLOGDIR)/ctriface_log.err &
	sudo env "PATH=$(PATH)" go test $(EXTRATESTFILES) $(EXTRAGOARGS)
	sudo env "PATH=$(PATH)" go test $(EXTRATESTFILES) $(EXTRAGOARGS) -args $(WITHUPF)
	./../scripts/clean_fcctr.sh

test-man:
	sudo ./clear_cache.sh
	./../scripts/clean_fcctr.sh
	# sudo mkdir -m777 -p $(CTRDLOGDIR) && sudo env "PATH=$(PATH)" /usr/local/bin/firecracker-containerd --config /etc/firecracker-containerd/config.toml 1>$(CTRDLOGDIR)/ctriface_log_noupf_man_travis.out 2>$(CTRDLOGDIR)/ctriface_log_noupf_man_travis.err &
	# sudo env "PATH=$(PATH)" go test $(EXTRAGOARGS) -run TestSnapLoadOnlyOne
	# ./../scripts/clean_fcctr.sh
	sudo mkdir -m777 -p $(CTRDLOGDIR) && sudo env "PATH=$(PATH)" /usr/local/bin/firecracker-containerd --config /etc/firecracker-containerd/config.toml 1>$(CTRDLOGDIR)/ctriface_log_upf_man_travis.out 2>$(CTRDLOGDIR)/ctriface_log_upf_man_travis.err &
	sudo env "PATH=$(PATH)" go test $(EXTRAGOARGS) -run TestSnapLoadOnlyOne -args $(WITHUPF) 
	# ./../scripts/clean_fcctr.sh
	# sudo mkdir -m777 -p $(CTRDLOGDIR) && sudo env "PATH=$(PATH)" /usr/local/bin/firecracker-containerd --config /etc/firecracker-containerd/config.toml 1>$(CTRDLOGDIR)/ctriface_log_lazy_man_travis.out 2>$(CTRDLOGDIR)/ctriface_log_lazy_man_travis.err &
	# sudo env "PATH=$(PATH)" go test $(EXTRAGOARGS) -run TestSnapLoadOnlyOne -args $(WITHUPF) $(WITHLAZY)
	# ./../scripts/clean_fcctr.sh

test-skip:
	sudo mkdir -m777 -p $(CTRDLOGDIR) && sudo env "PATH=$(PATH)" /usr/local/bin/firecracker-containerd --config /etc/firecracker-containerd/config.toml 1>$(CTRDLOGDIR)/ctriface_log_noupf_man_skip.out 2>$(CTRDLOGDIR)/ctriface_log_noupf_man_skip.err &
	sudo env "PATH=$(PATH)" go test $(EXTRAGOARGS) -run TestParallelSnapLoad
	sudo env "PATH=$(PATH)" go test $(EXTRAGOARGS) -run TestParallelPhasedSnapLoad
	./../scripts/clean_fcctr.sh
	sudo mkdir -m777 -p $(CTRDLOGDIR) && sudo env "PATH=$(PATH)" /usr/local/bin/firecracker-containerd --config /etc/firecracker-containerd/config.toml 1>$(CTRDLOGDIR)/ctriface_log_upf_man_skip.out 2>$(CTRDLOGDIR)/ctriface_log_upf_man_skip.err &
	sudo env "PATH=$(PATH)" go test $(EXTRAGOARGS) -run TestParallelSnapLoad -args $(WITHUPF)
	sudo env "PATH=$(PATH)" go test $(EXTRAGOARGS) -run TestParallelPhasedSnapLoad -args $(WITHUPF)
	./../scripts/clean_fcctr.sh
	sudo mkdir -m777 -p $(CTRDLOGDIR) && sudo env "PATH=$(PATH)" /usr/local/bin/firecracker-containerd --config /etc/firecracker-containerd/config.toml 1>$(CTRDLOGDIR)/ctriface_log_lazy_man_skip.out 2>$(CTRDLOGDIR)/ctriface_log_lazy_man_skip.err &
	sudo env "PATH=$(PATH)" go test $(EXTRAGOARGS) -run TestParallelSnapLoad -args $(WITHUPF) $(WITHLAZY)
	sudo env "PATH=$(PATH)" go test $(EXTRAGOARGS) -run TestParallelPhasedSnapLoad -args $(WITHUPF) $(WITHLAZY)
	./../scripts/clean_fcctr.sh


bench:
	sudo env "PATH=$(PATH)" go test $(BENCHFILES) $(GOBENCH)
	./../scripts/clean_fcctr.sh
.PHONY: test test-man test-man-upf bench

test-seqCSS:
	sudo ./clear_cache.sh
	./../scripts/clean_fcctr.sh
	sudo mkdir -m777 -p $(CTRDLOGDIR) && sudo env "PATH=$(PATH)" /usr/local/bin/firecracker-containerd --config /etc/firecracker-containerd/config.toml 1>$(CTRDLOGDIR)/phased.out 2>$(CTRDLOGDIR)/phased.err &
	sudo env "PATH=$(PATH)" go test $(EXTRAGOARGS) -run TestSequentialCSS -args $(WITHUPF) \
	-writeBW=$(writeBW) -isVmTouch=$(isVmTouch) -dumpMetrics=$(dumpMetrics) \
	-parallelNum=$(parallelNum) -interferNum=$(interferNum) -sameCtImg=$(sameCtImg) \
	# & sudo iotop -d 0.1 -t -o -k > ./BW_logs/fio/fsync/iotop_$(parallelNum)_$(interferNum)_$(writeBW).log \
	# & iostat -m -t 1 -p sda -c -o JSON > ./BW_logs/fio/fsync/iostat_$(parallelNum)_$(interferNum)_$(writeBW).json
	# python3 /home/cc/vhive/ctriface/get_iostat.py ./Seq_CreateSS_$(parallelNum)_$(interferNum)/iostat_$(parallelNum)_$(interferNum)
	# python3 /home/cc/vhive/ctriface/get_iotop.py ./Seq_CreateSS_$(parallelNum)_$(interferNum)/iotop_$(parallelNum)_$(interferNum)
	./kill_running_tools.sh

test-onlyCSS:
	sudo ./clear_cache.sh
	./../scripts/clean_fcctr.sh
	sudo mkdir -m777 -p $(CTRDLOGDIR) && sudo env "PATH=$(PATH)" /usr/local/bin/firecracker-containerd --config /etc/firecracker-containerd/config.toml 1>$(CTRDLOGDIR)/phased.out 2>$(CTRDLOGDIR)/phased.err &
	sudo env "PATH=$(PATH)" go test $(EXTRAGOARGS) -run TestOnlyCSS -args $(WITHUPF) \
	-writeBW=$(writeBW) -isVmTouch=$(isVmTouch) -dumpMetrics=$(dumpMetrics) \
	-parallelNum=$(parallelNum) -interferNum=$(interferNum) \
	# & iostat -m -t 1 -p sda -c -o JSON > ./only_CSS/iostat_$(interferNum)_$(writeBW).json \
	# & sudo iotop -d 0.1 -t -o -k > ./only_CSS/iotop_$(interferNum)_$(writeBW).json