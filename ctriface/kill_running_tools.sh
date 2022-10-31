# pidstatPids=$(ps -ef | grep pidstat | awk '{print $2}')
# for i in $pidstatPids; do
#   echo $i
#   sudo kill -9 $i
# done


iotopPids=$(ps -ef | grep iotop | awk '{print $2}')
# echo $pids
for i in $iotopPids; do
  echo $i
  sudo kill -9 $i
done


fioPids=$(ps -ef | grep fio | awk '{print $2}')
# echo $pids
for i in $fioPids; do
  echo $i
  sudo kill -9 $i
done

iostatPids=$(ps -ef | grep iostat | awk '{print $2}')
# echo $pids
for i in $iostatPids; do
  echo $i
  sudo kill -2 $i
done
