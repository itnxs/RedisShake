package main

import (
    "RedisShake/internal/config"
    "RedisShake/internal/entry"
    "RedisShake/internal/function"
    "RedisShake/internal/log"
    "RedisShake/internal/reader"
    "RedisShake/internal/status"
    "RedisShake/internal/writer"
    "context"
    "flag"
    "fmt"
    "os"
    "os/signal"
    "path/filepath"
    "runtime"
    "syscall"

    "github.com/mcuadros/go-defaults"
)

var file, host, username, password string

func init() {
    flag.StringVar(&file, "f", "", "rdb文件")
    flag.StringVar(&host, "h", "127.0.0.1:6379", "目标地址")
    flag.StringVar(&username, "u", "", "用户")
    flag.StringVar(&password, "p", "", "密码")
    flag.Parse()

    log.Console("info")

    file, err := filepath.Abs(file)
    if err != nil || filepath.Ext(file) != ".rdb" {
        log.Panicf("rdb file not found")
    }

    _, err = os.Stat(file)
    if err != nil || os.IsNotExist(err) {
        log.Panicf("rdb file not exist")
    }

    if len(host) <= 0 {
        log.Panicf("host is empty")
    }
}

func main() {
    v := config.DefaultConfig()
    runtime.GOMAXPROCS(runtime.NumCPU())

    // create reader
    var theReader reader.Reader
    if len(file) > 0 {
        opts := new(reader.RdbReaderOptions)
        opts.Filepath = file
        defaults.SetDefaults(opts)
        err := v.UnmarshalKey("rdb_reader", opts)
        if err != nil {
            log.Panicf("failed to read the RdbReader config entry. err: %v", err)
        }

        theReader = reader.NewRDBReader(opts)
        fmt.Println(theReader)
    }

    // create writer
    var theWriter writer.Writer
    if len(host) > 0 {
        opts := new(writer.RedisWriterOptions)
        opts.Address = host
        opts.Username = username
        opts.Password = password
        defaults.SetDefaults(opts)
        err := v.UnmarshalKey("redis_writer", opts)
        if err != nil {
            log.Panicf("failed to read the RedisStandaloneWriter config entry. err: %v", err)
        }
        if opts.OffReply && config.Opt.Advanced.RDBRestoreCommandBehavior == "panic" {
            log.Panicf("the RDBRestoreCommandBehavior can't be 'panic' when the server not reply to commands")
        }
        if opts.Cluster {
            theWriter = writer.NewRedisClusterWriter(opts)
            log.Infof("create RedisClusterWriter: %v", opts.Address)
        } else {
            theWriter = writer.NewRedisStandaloneWriter(opts)
            log.Infof("create RedisStandaloneWriter: %v", opts.Address)
        }

        // clean db
        e := entry.NewEntry()
        e.Argv = []string{"FLUSHALL"}
        theWriter.Write(e)
    }

    // create status
    status.Init(theReader, theWriter)

    log.Infof("start ...")

    ctx, cancel := context.WithCancel(context.Background())
    ch := theReader.StartRead(ctx)
    go waitShutdown(cancel)

    for e := range ch {
        // calc arguments
        e.Parse()
        status.AddReadCount(e.CmdName)

        // filter
        log.Debugf("function before: %v", e)
        entries := function.RunFunction(e)
        log.Debugf("function after: %v", entries)

        for _, en := range entries {
            en.Parse()
            theWriter.Write(en)
            status.AddWriteCount(en.CmdName)
        }
    }

    theWriter.Close() // Wait for all writing operations to complete
    log.Infof("all done")
}

func waitShutdown(cancel context.CancelFunc) {
    quitCh := make(chan os.Signal, 1)
    signal.Notify(quitCh, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
    sig := <-quitCh
    log.Infof("Got signal: %s to exit.", sig)
    cancel()
}
