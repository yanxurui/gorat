package main

import (
    "bufio"
    "flag"
    "fmt"
    "io"
    "log"
    "net"
    "strconv"
    "strings"
    "time"
)

var localAddr *string = flag.String("l", "0.0.0.0:8090", "local address")

type peer struct {
    addr string // This is the id of the peer
    conn net.Conn
    when_connected time.Time
    busy bool // This acts like a lock
    local bool
}

var peers = make(map[string]*peer)

func Copy(closer chan *peer, dst *peer, src *peer) {
    dst_addr := dst.addr
    src_addr := src.addr
    log.Printf("Begin %s<-%s", dst_addr, src_addr)
    // net.Conn implements io.Reader and io.Writer
    bytes_cnt, err := io.Copy(dst.conn, src.conn)
    if err != nil {
        log.Println("error copying data:", err)
        if neterr, ok := err.(net.Error); ok && neterr.Timeout() {
            log.Println("Timed out. Disable timeout for", src_addr)
            // 0 cancels timeout
            src.conn.SetReadDeadline(time.Time{})
        }
    }

    log.Printf("End %s<-%s, %d bytes transferred", dst_addr, src_addr, bytes_cnt)

    // Proxy is stopped, send signal to the parent goroutine
    closer <- src
}

func Proxy(local_peer *peer, remote_peer *peer) {
    local_peer_addr := local_peer.addr
    remote_peer_addr := remote_peer.addr
    log.Printf("Start port forwarding between %s and %s", local_peer_addr, remote_peer_addr)
    closer := make(chan *peer, 2)
    log.Println("local <- remote")
    go Copy(closer, local_peer, remote_peer)
    log.Println("local -> remote")
    go Copy(closer, remote_peer, local_peer)
    closed_peer := <- closer
    var opposite_peer *peer
    if closed_peer.addr == local_peer.addr {
        opposite_peer = remote_peer
    } else {
        // remote peer is disconnected, we will remove it in PrintRemotePeers
        opposite_peer = local_peer
    }
    // One end quits, need to interrupt or wake up the blocking read from the other end
    // Since io.Copy is not interruptible, a workaround is to set SetReadDeadline to make the read time out immediately
    log.Println("Interrupt the other end")
    opposite_peer.conn.SetReadDeadline(time.Now())
    log.Printf("End port forwarding between %s and %s", local_peer_addr, remote_peer_addr)
    <- closer
}

func HandleLocal(c net.Conn) {
    defer CloseConnection(c)
    local_peer := peer{
        addr: c.RemoteAddr().String(),
        when_connected: time.Now(),
        busy: false,
        local: true,
        conn: c}
    local_peer_addr := local_peer.addr
    log.Println("Handling local peer", local_peer_addr)
    fmt.Fprintln(c, "Hello", local_peer_addr)
    fmt.Fprintln(c, "Type q to quit")
    num_to_addr_mapping := PrintRemotePeers(c)
    scanner := bufio.NewScanner(c)
    for scanner.Scan() {
        line := scanner.Text()
        line = strings.TrimSpace(line)
        log.Printf("%s says: %s", local_peer_addr, line) // or do something else with line
        if line == ""{
            continue
        } else if line == "q" {
            break
        } else if i, err := strconv.Atoi(line); err == nil {
            if addr, ok := num_to_addr_mapping[i]; ok {
                if remote_peer, ok := peers[addr]; ok {
                    if !remote_peer.busy {
                        // local_peer is a struct whereas remote_peer is the pointer of a struct
                        remote_peer.busy = true
                        Proxy(&local_peer, remote_peer)
                        remote_peer.busy = false
                    } else {
                        fmt.Fprintf(c, "Sorry. The client you chose is busy now.\n")
                    }
                } else {
                    fmt.Fprintf(c, "Sorry. The client you chose does not exist or is just gone.\n")
                }
            }
        }
        num_to_addr_mapping = PrintRemotePeers(c)
    }
}

func HandleRemote(c net.Conn) {
    addr := c.RemoteAddr().String()
    log.Println("Handle remote peer", addr)
    peers[addr] = &peer{
        addr: addr,
        when_connected: time.Now(),
        busy: false,
        local: false,
        conn: c}
}

func IsLocal(c net.Conn) bool {
    if addr, ok := c.RemoteAddr().(*net.TCPAddr); ok {
        ip := addr.IP.String()
        switch ip {
        case
            "localhost",
            "::1",
            "127.0.0.1":
            return true
        }
        return false
    }
    log.Panicln("Unknown IP in ", c.RemoteAddr())
    return false
}

func CheckAlive(p *peer) bool {
    if p.busy {
        return true
    }
    p.busy = true
    c := p.conn
    log.Printf("Detect if peer %s is disconnected", p.addr)

    // Set timeout to avoid blocking on read
    c.SetReadDeadline(time.Now().Add(10 * time.Millisecond))
    defer func() {
        p.busy = false

        // Cancel timeout later
        var zero time.Time
        defer c.SetReadDeadline(zero)
    }()

    // If the connection is healthy, read all bytes available until a timeout is encountered
    // Otherwise, the conneciton is probably closed
    tmp := make([]byte, 256)
    for {
        n, err := c.Read(tmp)
        if err != nil {
            log.Println("Read error:", err)
            if err, ok := err.(net.Error); ok && err.Timeout() {
                // Timeout is expected
                break
            } else {
                // Other errors indicate disconnection
                log.Println(p.addr, "is disconnected")
                return false
            }
        }
        log.Println("Got", n, "bytes.")
    }

    return true
}

func CheckAliveForAll() {
    log.Println("Checking if remote peers are alive...")
    log.Println("Count of remote peers:", len(peers))
    for k, v := range peers {
        if !CheckAlive(v) {
            // It is safe to delete a key value pair while in a range
            log.Printf("Remove remote peer %s", k)
            delete(peers, k)
            CloseConnection(v.conn)
        }
    }
}

func PrintRemotePeers(c net.Conn) map[int]string{
    fmt.Fprintf(c, "Please enter the number to connect to a remote client.\n")
    CheckAliveForAll()
    i := 1
    num_to_addr_mapping := make(map[int]string)
    for k, v := range peers {
        num_to_addr_mapping[i] = k
        fmt.Fprintf(c, "%d: %s, %s, busy=%t\n", i, k, v.when_connected.Format("2006-01-02 15:04:05"), v.busy)
        i++
    }
    return num_to_addr_mapping
}

func CloseConnection(c net.Conn) {
    addr := c.RemoteAddr()
    log.Println("Closing connection", addr)

    if err := c.Close(); err != nil {
        log.Printf("Closing connection %s failed with error: %s", addr, err)
    }
}

func main() {
    flag.Parse()
    log.Printf("Listening: %v\n", *localAddr)

    listener, err := net.Listen("tcp", *localAddr)
    if err != nil {
        panic(err)
    }

    ticker := time.NewTicker(60 * time.Second)
    go func() {
        for range ticker.C {
            CheckAliveForAll()
        }
    }()

    for {
        conn, err := listener.Accept()
        log.Println("New connection", conn.RemoteAddr())
        if err != nil {
            log.Println("error accepting connection", err)
            continue
        }

        if IsLocal(conn) {
            go HandleLocal(conn)
        } else {
            defer CloseConnection(conn)
            go HandleRemote(conn)
        }
    }
}
