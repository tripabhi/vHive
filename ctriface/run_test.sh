#!/bin/bash

# for parallelNum in 4
# do
#    for writeBW in 2000
#    do
#       for interferNum in 4 8 16 32
#       do
#          for i in {1..10}
#          do
#             make test-seqCSS parallelNum=$parallelNum writeBW=$writeBW interferNum=$interferNum
#             echo "finished $parallelNum $interferNum $writeBW $i run"
#             sleep 10
#             # echo $w $i
#          done
#       done
#    done
# done

# the following is the without interference
for parallelNum in 4 8 16 32
do
   for writeBW in 99999
   do
      for interferNum in 0
      do
         for i in {1..10}
         do
            make test-seqCSS parallelNum=$parallelNum writeBW=$writeBW interferNum=$interferNum
            echo "finished $parallelNum $interferNum $writeBW $i run"
            sleep 10
            # echo $w $i
         done
      done
   done
done