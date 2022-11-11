for parallelNum in 1
do
   for writeBW in 100
   do
      for interferNum in 1 4 8 16
      do
         for i in {1..10}
         do
            make test-seqCSS parallelNum=$parallelNum writeBW=$writeBW interferNum=$interferNum
            echo "finished $w $i run"
            sleep 10
            # echo $w $i
         done
      done
   done
done