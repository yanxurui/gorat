package main

import (
    "bufio"
    "fmt"
    "github.com/stretchr/testify/assert"
    "net"
    "os"
    "regexp"
    "strings"
    "sync"
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
    fmt.Println("seen in local chat:")
    return seeInChannel(c.msgs, keyword)
}

func (c *client) find(keyword string) (string, bool) {
    fmt.Println("found in local chat:")
    return filterLine(c.msgs, keyword)
}

func (c *client) close() {
    if c != nil && c.conn != nil {
        fmt.Println("closing connection...")
        c.conn.Close()
        c.conn = nil
    }
}


func seeInLog(keyword string) bool {
    fmt.Println("seen in server log:")
    return seeInChannel(lines, keyword)
}

func seeInChannel(ch <-chan string, keyword string) bool {
    _, seen := filterLine(ch, keyword)
    return seen
}

func filterLine(ch <-chan string, keyword string) (string, bool) {
    for {
        // non-blocking channel read with timeout
        select {
        case line, more := <-ch:
            if more {
                fmt.Println("\t", line)
                // check if the keyword exists in lines we received
                if strings.Contains(line, keyword) {
                    return line, true
                }
            } else {
                fmt.Println("channel closed")
                return "", false
            }
        case <-time.After(3 * time.Second):
            fmt.Printf("Couldn't find %q\n", keyword)
            return "", false
        }
    }
}

func connect() *client {
    conn, err := net.Dial("tcp", "127.0.0.1:8091")
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

func connectAsLocal() *client {
    fmt.Println("connecting as local...")
    c := connect()
    if c != nil {
        seeInLog("Handling local")
    }
    return c
}

func connectAsRemote() *client {
    fmt.Println("connecting as remote...")
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

    l := connectAsLocal()
    defer l.close()
    l.send("1")
    assert.True(l.see("Connected"))
    l.send("ping")
    assert.True(r.see("ping"))

    r.send("pong")
    assert.True(l.see("pong"))
}

// Validate a local peer can still connect to other remote
// peers when the current remote peer disconnected
func TestRemoteDisconnected(t *testing.T) {
    assert := assert.New(t)
    r1 := connectAsRemote()
    defer r1.close()

    l := connectAsLocal()
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

// Validate the remote peer can still be connected by other local peers
// when some local peer disconnected
func TestLocalDisconnected(t *testing.T) {
    assert := assert.New(t)
    r := connectAsRemote()
    defer r.close()

    // l1 disconnected
    l1 := connectAsLocal()
    defer l1.close()
    l1.send("1")
    assert.True(l1.see("Connected"))
    l1.close()
    seeInLog("Closing connection")

    // l2 should still be able to connect to r
    l2 := connectAsLocal()
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

    l1 := connectAsLocal()
    defer l1.close()
    l1.send("1")
    assert.True(l1.see("Connected"))

    l2 := connectAsLocal()
    defer l2.close()
    l2.send("1")
    assert.True(l2.see("is busy now"))
}

// Validate the remote peers are sorted by connected time
func TestSort(t *testing.T) {
    assert := assert.New(t)
    r1 := connectAsRemote()
    defer r1.close()
    time.Sleep(1 * time.Second)
    r2 := connectAsRemote()
    defer r2.close()

    l := connectAsLocal()
    defer l.close()

    l1, ok1 := l.find("1: ")
    assert.True(ok1)
    l2, ok2 := l.find("2: ")
    assert.True(ok2)
    re := regexp.MustCompile(`\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}`)
    m1 := re.FindStringSubmatch(l1)
    assert.NotNil(m1)
    m2 := re.FindStringSubmatch(l2)
    assert.NotNil(m1)
    assert.True(m1[0] < m2[0])
}

// Test many remote peers are connecting at the same time
func TestConcurrentConnections(t *testing.T) {
    N := 50 // only 6 threads when N = 100
    assert := assert.New(t)

    remote_connections := make([]*client, N)
    t.Cleanup(func(){
        fmt.Println("tear-down code")
        for _, c := range remote_connections {
            c.close()
        }
    })

    var wg sync.WaitGroup
    start := time.Now()
    for i := 0; i < N; i++ {
        fmt.Println("connection #", i+1)
        wg.Add(1)
        i := i
        go func() {
            defer wg.Done()
            remote_connections[i] = connectAsRemote()
        }()
    }
    wg.Wait()
    duration := time.Since(start)

    // Formatted string, such as "2h3m0.5s" or "4.503μs"
    fmt.Println(duration)

    l := connectAsLocal()
    defer l.close()
    assert.True(l.see(fmt.Sprintf("%d: ", N)))
}


// global initialization method
func setup() {
    fmt.Println("setup")
    // start up the server process
    // go run . will launch a child process to run the main module
    // however, cancel will only kill the parent process
    // lines = d.Start("go", "run", ".")
    lines = d.Start("./gorat", "-l", "127.0.0.1:8091")
    if !seeInLog("Listening") {
        shutdown()
        panic("Failed to start the server")
    }
}

// global cleanup method
// Todo: this is not run if any test method panics because m.Run is not returned
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

