#!/bin/bash

TEST_DIR=/var/lib/fio_space/
RESULT_DIR=./test_diffCSSNC/fio/8GB/
CGROUP_PROC=/sys/fs/cgroup/test/cgroup.procs

function add_children_to_cgroup() {
    CHILDREN=`pgrep -P $1|xargs`
    for CPID in $CHILDREN;
    do
        echo $CPID | sudo tee $2;
    done
}

sudo fio --name=write_throughput --directory=$TEST_DIR --numjobs=1 \
    --size=8G --time_based --runtime=250s --ramp_time=2s --ioengine=libaio \
    --direct=0 --verify=0 --end_fsync=1 --bs=4k --iodepth=32 --rw=write \
    --group_reporting=1 & FIO_PID=$!

# --output=$RESULT_DIR/fio_stats
# sudo fio fio_test.fio & FIO_PID=$!
echo $FIO_PID
pgrep -P $FIO_PID
# echo $FIO_PID | sudo tee /sys/fs/cgroup/test/cgroup.procs;

add_children_to_cgroup $FIO_PID $CGROUP_PROC;