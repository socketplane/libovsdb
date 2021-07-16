package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"time"

	"github.com/ovn-org/libovsdb/client"
	"github.com/ovn-org/libovsdb/example/vswitchd"
	"github.com/ovn-org/libovsdb/model"
	"github.com/ovn-org/libovsdb/ovsdb"
	"github.com/ovn-org/libovsdb/server"
)

var (
	cpuprofile = flag.String("cpuprofile", "", "write cpu profile to this file")
	memprofile = flag.String("memoryprofile", "", "write memory profile to this file")
	port       = flag.Int("port", 56640, "tcp port to listen on")
)

func main() {
	flag.Parse()
	var err error
	if *cpuprofile != "" {
		f, err := os.Create(*cpuprofile)
		if err != nil {
			log.Fatal(err)
		}
		if err := pprof.StartCPUProfile(f); err != nil {
			log.Fatal(err)
		}
		defer pprof.StopCPUProfile()
	}

	dbModel, err := vswitchd.FullDatabaseModel()
	if err != nil {
		log.Fatal(err)
	}
	wd, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}
	path := filepath.Join(wd, "vswitchd", "vswitchd.ovsschema")
	f, err := os.Open(path)
	if err != nil {
		log.Fatal(err)
	}
	schema, err := ovsdb.SchemaFromFile(f)
	if err != nil {
		log.Fatal(err)
	}

	ovsDB := server.NewInMemoryDatabase(map[string]*model.DBModel{
		schema.Name: dbModel,
	})

	s, err := server.NewOvsdbServer(ovsDB, server.DatabaseModel{
		Model:  dbModel,
		Schema: schema,
	})
	if err != nil {
		log.Fatal(err)
	}
	defer s.Close()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)

	go func(o *server.OvsdbServer) {
		if err := o.Serve("tcp", fmt.Sprintf(":%d", *port)); err != nil {
			log.Fatal(err)
		}
	}(s)

	time.Sleep(1 * time.Second)
	c, err := client.NewOVSDBClient(dbModel, client.WithEndpoint(fmt.Sprintf("tcp::%d", *port)))
	if err != nil {
		log.Fatal(err)
	}

	err = c.Connect(context.Background())
	if err != nil {
		log.Fatal(err)
	}
	ovsRow := &vswitchd.OpenvSwitch{
		UUID: "ovs",
	}
	ovsOps, err := c.Create(ovsRow)
	if err != nil {
		log.Fatal(err)
	}
	reply, err := c.Transact(context.Background(), ovsOps...)
	if err != nil {
		log.Fatal(err)
	}
	_, err = ovsdb.CheckOperationResults(reply, ovsOps)
	if err != nil {
		log.Fatal(err)
	}
	c.Close()
	log.Printf("listening on tcp::%d", *port)
	<-sig

	if *memprofile != "" {
		f, err := os.Create(*memprofile)
		if err != nil {
			log.Fatal(err)
		}
		defer f.Close()
		runtime.GC()
		if err := pprof.WriteHeapProfile(f); err != nil {
			log.Fatal("could not write memory profile: ", err)
		}
	}
}
