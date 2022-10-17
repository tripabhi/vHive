for parallelNum in 1 2 4 8 16
do
   for writeBW in 99999
   do
      for interferNum in 1 2 4 8
      do
         for i in {1..8}
         do
            make test-seqCSS parallelNum=$parallelNum writeBW=$writeBW interferNum=$interferNum sameCtImg=true
            echo "finished $w $i run"
            sleep 2
            # echo $w $i
         done
      done
   done
done