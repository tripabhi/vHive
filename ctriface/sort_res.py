import pandas as pd

dataFrame = pd.read_csv("./verify/result.csv")
dataFrame.sort_values("ExpEnv", axis=0, ascending=False, inplace=True, na_position='first')
# print(dataFrame)
dataFrame.to_csv("./verify/sorted.csv", index=False)