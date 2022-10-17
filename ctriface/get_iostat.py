import json
import pandas as pd
import sys

infile_prefix = sys.argv[1]
with open('./BW_logs/iostat_' + infile_prefix + '.json') as f:
   data = json.load(f)

# print(data)
timelist = data['sysstat']['hosts'][0]['statistics']
# print(timelist)
timestamp = []
user = []
system = []
iowait = []
total = []
disk_read = []
disk_write = []

for item in timelist:
    timestamp.append(item["timestamp"][-5:])
    user.append(item["avg-cpu"]["user"])
    system.append(item["avg-cpu"]["system"])
    iowait.append(item["avg-cpu"]["iowait"])
    total.append(round(100 - item["avg-cpu"]["idle"], 2))
    disk_read.append(item['disk'][0]['MB_read/s'])
    disk_write.append(item['disk'][0]['MB_wrtn/s'])

print(timestamp)
print(user)
print(total)
print(iowait)
print(system)
print(disk_read)
print(disk_write)

#write to csv
# fields = ['Timestamp', 'user', 'system', 'iowait', 'total']
# dict = {'Timestamp': timestamp, }
dict = {'Timestamp': timestamp, 'user': user, 'system': system, 'iowait': iowait, 'Total CPU': total, 'iostat read (MB/s)': disk_read, 'iostat write (MB/s)': disk_write}
df = pd.DataFrame(dict)
df.to_csv('./BW_logs/iostat_' + infile_prefix + ".csv", index=False)