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

func (c *client) Close() error {
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

func TestBusy(t *testing.T) {
    assert := assert.New(t)
    r := connectAsRemote()
    defer r.Close()
    l1 := connect()
    defer l1.Close()
    l2 := connect()
    defer l2.Close()
    l1.send("1")
    l2.send("1")
    assert.True(l2.see("Sorry"))
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
    fmt.Println("shutdown")
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

