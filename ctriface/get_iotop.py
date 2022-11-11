import re
import csv
import pandas as pd
import sys

infile_prefix = sys.argv[1]
base_dir = './BW_logs/1024_concurrent_CSS/'

disk_write_phrase = "Current DISK WRITE: "
disk_read_phrase = "Current DISK READ: "
write_bw_all = []
read_bw_all = []
timestamp_all = []
with open(base_dir + 'iotop_' + infile_prefix + '.log', "r") as f:
    for line in f:
        if disk_write_phrase in line:
            # start = line.index(disk_write_phrase)+ len(disk_write_phrase)
            # later_string = line[start:]
            cur_string = re.findall(r"[-+]?(?:\d*\.\d+|\d+)", line)
            timestamp_all.append(cur_string[0]+":"+cur_string[1]+":"+cur_string[2])
            read_bw_all.append(round(float(cur_string[3])/1024, 2))
            write_bw_all.append(round(float(cur_string[4])/1024, 2))
            # write_bw_all.append()

print(timestamp_all)
print(read_bw_all)
print(write_bw_all)
print(len(timestamp_all))
print(len(read_bw_all))
print(len(write_bw_all))

#write to csv
# fields = ['Timestamp', 'Disk Read (MB/s)', 'Disk Write (MB/s)']
dict = {'Timestamp': timestamp_all, 'iotop read (MB/s)': read_bw_all, 'iotop write (MB/s)': write_bw_all}
df = pd.DataFrame(dict)
df.to_csv(base_dir + 'iotop_' + infile_prefix + ".csv", index=False)