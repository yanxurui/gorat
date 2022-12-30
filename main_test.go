package main

import (
    "bufio"
    "fmt"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/suite"
    "net"
    "regexp"
    "strings"
    "sync"
    "testing"
    "time"
)

type client struct {
    conn net.Conn
    msgs <-chan string
    local bool
    suite *MainTestSuite
}

func (c *client) send(s string) {
    fmt.Fprintln(c.conn, s)
}

func (c *client) see(keyword string) bool {
    c.suite.T().Log("looking for", keyword, "in local chat:")
    _, seen := c.suite.filterLines(c.msgs, keyword)
    return seen
}

func (c *client) find(keyword string) (string, bool) {
    c.suite.T().Log("Find line containing", keyword, "in local chat:")
    return c.suite.filterLines(c.msgs, keyword)
}

func (c *client) close() {
    if c != nil && c.conn != nil {
        c.suite.T().Log("closing connection...")
        c.conn.Close()
        c.conn = nil
        if c.local {
            c.suite.seeInLog("Closing connection")
        } else {
            // server is not aware immediately when the remote peer disconnects
        }
    }
}


type MainTestSuite struct {
    suite.Suite
    d Daemon
    lines <-chan string
    clients []*client
}

// run once before/after the entire test suite
func (suite *MainTestSuite) SetupSuite() {
    suite.T().Log("SetupSuite...")
    // start up the server process
    // `go run .` does not work because it will launch a child process to run the main module
    // however, cancel will only kill the parent process
    // lines = suite.d.Start("go", "run", ".")
    suite.lines = suite.d.Start("./gorat", "-l", "127.0.0.1:8091")
    if !suite.seeInLog("Listening") {
        panic("Failed to start the server")
    }
    go func() {
        for m := range suite.d.log {
            suite.T().Log(m)
        }
    }()
}

// Todo: this is not run if any test method panics because m.Run is not returned
func (suite *MainTestSuite) TearDownSuite() {
    suite.T().Log("TearDownSuite...")
    // shutdown the server process
    suite.d.Cancel()
    <-suite.d.Done()
    suite.T().Log("CmdErr():", suite.d.CmdErr())
}

// run before/after each test in the suite
func (suite *MainTestSuite) SetupTest() {
    suite.T().Log("SetupTest...")
    suite.clients = []*client{}
}

func (suite *MainTestSuite) TearDownTest() {
    suite.T().Log("TearDownTest...")
    for _, c := range suite.clients {
        c.close()
    }
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestMainTestSuite(t *testing.T) {
    suite.Run(t, new(MainTestSuite))
}

// -----CUTOMIZED METHODS-----
func (suite *MainTestSuite) connect() *client {
    conn, err := net.Dial("tcp", "127.0.0.1:8091")
    if err != nil {
        suite.T().Fatal("error dialing remote addr", err)
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
            suite.T().Log(err)
        }
        close(msgs)
        suite.T().Log("connection closed")
    }()
    c := &client{conn: conn, msgs: msgs, suite: suite}
    suite.clients = append(suite.clients, c)
    return c
}

func (suite *MainTestSuite) connectAsLocal() *client {
    suite.T().Log("connecting as local...")
    c := suite.connect()
    if c != nil {
        suite.seeInLog("Handling local")
        c.local = true
    }
    return c
}

func (suite *MainTestSuite) connectAsRemote() *client {
    suite.T().Log("connecting as remote...")
    c := suite.connect()
    if c != nil {
        c.send("delevate")
        suite.seeInLog("Handling remote")
        c.local = false
    }
    return c
}

func (suite *MainTestSuite) seeInLog(keyword string) bool {
    suite.T().Log("looking for", keyword, "in server log:")
    _, seen := suite.filterLines(suite.lines, keyword)
    return seen
}

func (suite *MainTestSuite) filterLines(ch <-chan string, keyword string) (string, bool) {
    for {
        // non-blocking channel read with timeout
        select {
        case line, more := <-ch:
            if more {
                suite.T().Log("\t", line)
                // check if the keyword exists in lines we received
                if strings.Contains(line, keyword) {
                    return line, true
                }
            } else {
                suite.T().Log("channel closed")
                return "", false
            }
        case <-time.After(3 * time.Second):
            suite.T().Logf("Couldn't find %q\n", keyword)
            return "", false
        }
    }
}

// -----TEST METHODS-----
// Validate that a local peer can talk with a remote peer
func (suite *MainTestSuite) TestPingPong() {
    assert := assert.New(suite.T())
    r := suite.connectAsRemote()

    l := suite.connectAsLocal()
    l.send("1")
    assert.True(l.see("Connected"))
    l.send("ping")
    assert.True(r.see("ping"))

    r.send("pong")
    assert.True(l.see("pong"))
}

// Validate a local peer can still connect to other remote
// peers when the current remote peer disconnected
func (suite *MainTestSuite) TestRemoteDisconnected() {
    assert := assert.New(suite.T())
    r1 := suite.connectAsRemote()

    l := suite.connectAsLocal()
    l.send("1")
    assert.True(l.see("Connected"))

    // r1 disconnected
    r1.close()

    suite.connectAsRemote()

    // l should still be able to connect to r2
    l.send("l")
    assert.True(l.see("busy=false"))
    l.send("1")
    assert.True(l.see("Connected"))
}

// Validate the remote peer can still be connected by other local peers
// when some local peer disconnected
func (suite *MainTestSuite) TestLocalDisconnected() {
    assert := assert.New(suite.T())
    suite.connectAsRemote()

    // l1 disconnected
    l1 := suite.connectAsLocal()
    l1.send("1")
    assert.True(l1.see("Connected"))

    l1.close() // this will wait until 'Closing connection' is seen in the server's log

    // l2 should still be able to connect to r
    l2 := suite.connectAsLocal()
    assert.True(l2.see("busy=false"))
    l2.send("1")
    assert.True(l2.see("Connected"))
}

// Validate that if a local peer is connected with a remote peer
// no other local peers can connect to this remote peer
func (suite *MainTestSuite) TestBusy() {
    assert := assert.New(suite.T())
    suite.connectAsRemote()

    l1 := suite.connectAsLocal()
    l1.send("1")
    assert.True(l1.see("Connected"))

    l2 := suite.connectAsLocal()
    l2.send("1")
    assert.True(l2.see("is busy now"))
}

// Validate the remote peers are sorted by connected time
func (suite *MainTestSuite) TestSort() {
    assert := assert.New(suite.T())
    suite.connectAsRemote()
    time.Sleep(1 * time.Second)
    suite.connectAsRemote()

    l := suite.connectAsLocal()

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
func (suite *MainTestSuite) TestConcurrentConnections() {
    // suite.T().Skip()
    t := suite.T()
    N := 10 // only 6 threads when N = 100
    assert := assert.New(t)

    var wg sync.WaitGroup
    start := time.Now()
    for i := 0; i < N; i++ {
        suite.T().Log("connection #", i+1)
        wg.Add(1)
        go func() {
            defer wg.Done()
            suite.connectAsRemote()
        }()
    }
    wg.Wait()
    duration := time.Since(start)
    suite.T().Log("Time elapsed:", duration)

    l := suite.connectAsLocal()
    assert.True(l.see(fmt.Sprintf("%d: ", N)))
}

