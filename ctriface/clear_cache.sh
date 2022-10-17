#!/bin/bash

# sudo /bin/sh -c sync; echo 1 | sudo tee /proc/sys/vm/drop_caches
# sudo /bin/sh -c sync; echo 2 | sudo tee /proc/sys/vm/drop_caches
# sudo /bin/sh -c sync; echo 3 | sudo tee /proc/sys/vm/drop_caches

# Run a sync to reduce dirty caches
sudo sync

# Tell the OS to clear caches
sudo sh -c "/usr/bin/echo 3 > /proc/sys/vm/drop_caches"