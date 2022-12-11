import pandas as pd

dataFrame = pd.read_csv("./large_intf_small_victim/result.csv")
dataFrame.sort_values("ExpEnv", axis=0, ascending=False, inplace=True, na_position='first')
# print(dataFrame)
dataFrame.to_csv("./large_intf_small_victim/sorted.csv", index=False)