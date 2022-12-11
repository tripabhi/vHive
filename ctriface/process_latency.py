import glob
import os
import re
import pandas as pd
import csv

resultDir = "./large_intf_small_victim"
os.chdir(resultDir+"/")
all_rows = []
col_name = ["ExpEnv", "FcCreateVM", "FcCreateVMStd", "FcCreateVMMin", "FcCreateVMMax", \
            "NewContainer", "NewContainerStd", "NewContainerMin", "NewContainerMax", \
            "NewTask", "NewTaskStd", \
            "TaskStart", "TaskStartStd", \
            "TaskWait", "TaskWaitStd", "Total", "TotalStd"]
for file in glob.glob("*.csv"):
    cur_csv_df = pd.read_csv(file)
    cur_csv_prefix = re.search('(.*).csv', file).group(1)
    # all divide 1000 to ms
    cur_csv_df['FcCreateVM'] = cur_csv_df['FcCreateVM'].div(1000).round(3)
    # cur_csv_df['GetImage'] = cur_csv_df['GetImage'].div(1000).round(3)
    cur_csv_df['NewContainer'] = cur_csv_df['NewContainer'].div(1000).round(3)
    cur_csv_df['NewTask'] = cur_csv_df['NewTask'].div(1000).round(3)
    # cur_csv_df['StartVM'] = cur_csv_df['StartVM'].div(1000).round(3)
    cur_csv_df['TaskStart'] = cur_csv_df['TaskStart'].div(1000).round(3)
    cur_csv_df['TaskWait'] = cur_csv_df['TaskWait'].div(1000).round(3)
    cur_csv_df['Total'] = cur_csv_df['Total'].div(1000).round(3)

    # get mean
    FcCreateVMMean = cur_csv_df["FcCreateVM"].mean().round(0)
    # GetImageMean = cur_csv_df["GetImage"].mean().round(0)
    NewContainerMean = cur_csv_df["NewContainer"].mean().round(0)
    NewTaskMean = cur_csv_df["NewTask"].mean().round(0)
    # StartVMMean = cur_csv_df["StartVM"].mean().round(0)
    TaskStartMean = cur_csv_df["TaskStart"].mean().round(0)
    TaskWaitMean = cur_csv_df["TaskWait"].mean().round(0)
    TotalMean = cur_csv_df["Total"].mean().round(0)

    # get std
    FcCreateVMStd = cur_csv_df["FcCreateVM"].std().round(0)
    # GetImageStd = cur_csv_df["GetImage"].std().round(0)
    NewContainerStd = cur_csv_df["NewContainer"].std().round(0)
    NewTaskStd = cur_csv_df["NewTask"].std().round(0)
    # StartVMStd = cur_csv_df["StartVM"].std().round(0)
    TaskStartStd = cur_csv_df["TaskStart"].std().round(0)
    TaskWaitStd = cur_csv_df["TaskWait"].std().round(0)
    TotalStd = cur_csv_df["Total"].std().round(0)

    #get min and max
    FcCreateVMMin = cur_csv_df["FcCreateVM"].min().round(0)
    FcCreateVMMax = cur_csv_df["FcCreateVM"].max().round(0)
    NewContainerMin = cur_csv_df["NewContainer"].min().round(0)
    NewContainerMax = cur_csv_df["NewContainer"].max().round(0)

    cur_csv_row = [cur_csv_prefix, FcCreateVMMean, FcCreateVMStd, FcCreateVMMin, FcCreateVMMax, \
                    NewContainerMean, NewContainerStd, NewContainerMin, NewContainerMax, \
                    NewTaskMean, NewTaskStd, \
                    TaskStartMean, TaskStartStd, \
                    TaskWaitMean, TaskWaitStd, TotalMean, TotalStd]
    print(cur_csv_prefix, ":", cur_csv_row)
    all_rows.append(cur_csv_row)

with open('result.csv', 'w') as f:
    write = csv.writer(f)
    write.writerow(col_name)
    write.writerows(all_rows)