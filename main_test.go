package main

import (
    "bufio"
    "fmt"
    "github.com/stretchr/testify/assert"
    "net"
    "os"
    "strings"
    "testing"
    "time"
)

var d Daemon
var lines <-chan string

type client struct {
    conn net.Conn
    msgs <-chan string
}

func (c *client) send(s string) {
    fmt.Fprintln(c.conn, s)
}

func (c *client) see(keyword string) bool {
    return seeInChannel(c.msgs, keyword)
}

func (c *client) close() error {
    return c.conn.Close()
}

func seeInLog(keyword string) bool {
    return seeInChannel(lines, keyword)
}

func seeInChannel(ch <-chan string, keyword string) bool {
    for {
        // non-blocking channel read with timeout
        select {
        case line := <-ch:
            fmt.Println("see:", line)
            // check if the keyword exists in lines we received
            if strings.Contains(line, keyword) {
                return true
            }
        case <-time.After(3 * time.Second):
            fmt.Printf("Couldn't find %q\n", keyword)
            return false
        }
    }
}

func connect() *client {
    conn, err := net.Dial("tcp", "127.0.0.1:8090")
    if err != nil {
        fmt.Println("error dialing remote addr", err)
        return nil
    }

    // use buffering to prevent server side from being blocked
    msgs := make(chan string, 100)
    go func() {
        scanner := bufio.NewScanner(conn)
        for scanner.Scan() {
            msgs <- scanner.Text()
        }
        if err := scanner.Err(); err != nil {
            fmt.Println(err)
        }
        close(msgs)
        fmt.Println("connection closed")
    }()
    return &client{conn: conn, msgs: msgs}
}

func connectAsRemote() *client {
    c := connect()
    if c != nil {
        c.send("delevate")
        seeInLog("Handle remote")
    }
    return c
}

// Validate that a local peer can talk with a remote peer
func TestPingPong(t *testing.T) {
    assert := assert.New(t)
    r := connectAsRemote()
    defer r.close()

    l := connect()
    defer l.close()
    l.send("1")
    assert.True(l.see("Connected"))
    l.send("ping")
    assert.True(r.see("ping"))

    r.send("pong")
    assert.True(l.see("pong"))
}

// Validate the remote peer can still be connected when
// the local peer disconnected
func TestLocalDisconnected(t *testing.T) {
    assert := assert.New(t)
    r1 := connectAsRemote()
    defer r1.close()

    l := connect()
    defer l.close()
    l.send("1")
    assert.True(l.see("Connected"))

    // r1 disconnected
    r1.close()
    seeInLog("Closing connection")

    r2 := connectAsRemote()
    defer r2.close()

    // l should still be able to connect to r2
    l.send("l")
    assert.True(l.see("busy=false"))
    l.send("1")
    assert.True(l.see("Connected"))
}

// Validate a local peer can still connect to other remote
// peers when the remote peer disconnected
func TestRemoteDisconnected(t *testing.T) {
    assert := assert.New(t)
    r := connectAsRemote()
    defer r.close()

    // l1 disconnected
    l1 := connect()
    defer l1.close()
    l1.send("1")
    assert.True(l1.see("Connected"))
    l1.close()
    seeInLog("Closing connection")

    // l2 should still be able to connect to r
    l2 := connect()
    defer l2.close()
    assert.True(l2.see("busy=false"))
    l2.send("1")
    assert.True(l2.see("Connected"))
}

// Validate that if a local peer is connected with a remote peer
// no other local peers can connect to this remote peer
func TestBusy(t *testing.T) {
    assert := assert.New(t)
    r := connectAsRemote()
    defer r.close()

    l1 := connect()
    defer l1.close()
    l1.send("1")
    assert.True(l1.see("Connected"))

    l2 := connect()
    defer l2.close()
    l2.send("1")
    assert.True(l2.see("is busy now"))
}

func setup() {
    fmt.Println("setup")
    // start up the server process
    // go run . will launch a child process to run the main module
    // however, cancel will only kill the parent process
    // lines = d.Start("go", "run", ".")
    lines = d.Start("./gorat")
    if !seeInLog("Listening") {
        shutdown()
        panic("Failed to start the server")
    }
}

func shutdown() {
    fmt.Println("Shutdown...")
    // shutdown the server process
    d.Cancel()
    <-d.Done()
    fmt.Println("d.CmdErr():", d.CmdErr())
}

func TestMain(m *testing.M) {
    setup()
    code := m.Run()
    shutdown()
    os.Exit(code)
}

