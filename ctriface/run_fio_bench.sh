
BW_CAPS=(50 2)
# 
for BW_CAP in ${BW_CAPS[@]}
do
    for i in {1..5}
    do
        make test-seqCSS parallelNum=1 interferNum=0 writeBW=$BW_CAP
    done
done
