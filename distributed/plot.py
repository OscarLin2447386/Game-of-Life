import pandas as pd
import matplotlib.pyplot as plt

df = pd.read_csv("results.csv", delimiter=",", usecols=[0, 1], names=["name", "time_sec_op"], skiprows=1)


df['time_sec_op'] = pd.to_numeric(df['time_sec_op'], errors='coerce') 


names = df['name']
times = df['time_sec_op']

plt.figure(figsize=(10, 6))
plt.bar(names, times, color='red')
plt.xlabel('Test Configuration')
plt.ylabel('Time per Operation (sec/op)')
plt.title('Benchmark Results')
plt.xticks(rotation=90, ha="right")
plt.tight_layout()
plt.show()
