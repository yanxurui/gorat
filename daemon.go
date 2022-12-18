package main

import (
    "bufio"
    "context"
    "fmt"
    "io"
    "log"
    "os/exec"
)

type Daemon struct {
    cancel func()
    doneCh chan struct{}
    cmdErr error
}

func (d *Daemon) Start(command string, args ...string) <- chan string {
    ctx, cancel := context.WithCancel(context.Background())
    d.cancel = cancel

    d.doneCh = make(chan struct{})

    cmd := exec.CommandContext(ctx, command, args...)

    outR, outW := io.Pipe()
    // mw := io.MultiWriter(outW, os.Stdout)
    mw := io.MultiWriter(outW)
    cmd.Stdout = mw
    cmd.Stderr = mw

    // MultiWriter could be blocked by one of the writters.
    // Use buffering to prevent MultiWriter from being blocked.
    lines := make(chan string, 100)

    // Output lines producer goroutine.
    // This goroutine typically exits when there the write
    // end of the pipe is closed (which happens in the cmd.Wait goroutine).
    go func() {
        defer close(lines) // close on exit: when the scanner has no more to read, or has encountered an error.
        scanner := bufio.NewScanner(outR)
        for scanner.Scan() {
            lines <- scanner.Text()
        }
        if err := scanner.Err(); err != nil {
            // TODO: handle error.
        }
    }()

    // Start the command.
    if err := cmd.Start(); err != nil {
        log.Fatal(err)
    }

    // Goroutine that waits for the command to exit using cmd.Wait().
    // It closes the doneCh to indicate to users of Daemon that
    // the command has finished.
    // It closes the write end of the pipe to free-up the output lines producer
    // and, in turn, the output lines consumer goroutines.
    //
    // This goroutine must be run only after cmd.Start() returns,
    // otherwise cmd.Run() may panic.
    //
    // The command can exit either:
    // * normally with success;
    // * with failure due to a command error; or
    // * with failure due to Context cancelation.
    go func() {
        fmt.Println("waiting...")
        err := cmd.Wait()
        fmt.Println("command exited; error is:", err)
        _ = outW.Close() // TODO: handle error from Close(); log it maybe.
        d.cmdErr = err
        close(d.doneCh)
    }()

    return lines
}

// Done returns a channel, which is closed when the command
// started by Start exits: either normally, due to a command error,
// or by due to d.cancel.
// After the channel is closed, d.CmdErr() returns the error, if any,
// from the command's exit.
func (d *Daemon) Done() <-chan struct{} {
    fmt.Println("Done")
    return d.doneCh
}

// CmdErr returns the error, if any, from the command's exit.
// Only valid after the channel returned by Done() has been closed.
func (d *Daemon) CmdErr() error {
    return d.cmdErr
}

// Cancel causes the running command to exit preemptively.
// If Cancel is called after the command has already
// exited either naturally or due to a previous Cancel call,
// then Cancel has no effect.
func (d *Daemon) Cancel() {
    fmt.Println("Cancel")
    d.cancel()
}
