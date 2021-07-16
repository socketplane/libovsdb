package main

import (
	"context"
	"flag"
	"fmt"
	"log"

	"github.com/ovn-org/libovsdb/cache"
	"github.com/ovn-org/libovsdb/client"
	"github.com/ovn-org/libovsdb/example/vswitchd"
	"github.com/ovn-org/libovsdb/model"
	"github.com/ovn-org/libovsdb/ovsdb"
)

// Silly game that detects creation of Bridge named "stop" and exits
// Just a demonstration of how an app can use libovsdb library to configure and manage OVS
const (
	bridgeTable = "Bridge"
	ovsTable    = "Open_vSwitch"
)

var quit chan bool
var update chan model.Model

var rootUUID string
var connection = flag.String("ovsdb", "unix:/var/run/openvswitch/db.sock", "OVSDB connection string")

func play(ctx context.Context, ovs client.Client) {
	go processInput(ctx, ovs)
	for model := range update {
		bridge := model.(*vswitchd.Bridge)
		if bridge.Name == "stop" {
			fmt.Printf("Bridge stop detected: %+v\n", *bridge)
			ovs.Disconnect()
			quit <- true
		} else {
			fmt.Printf("Current list of bridges:\n")
			var bridges []vswitchd.Bridge
			if err := ovs.List(&bridges); err != nil {
				log.Fatal(err)
			}
			for _, b := range bridges {
				fmt.Printf("UUID: %s  Name: %s\n", b.UUID, b.Name)
			}
		}
	}
}

func createBridge(ctx context.Context, ovs client.Client, bridgeName string) {
	bridge := vswitchd.Bridge{
		UUID: "gopher",
		Name: bridgeName,
	}
	insertOp, err := ovs.Create(&bridge)
	if err != nil {
		log.Fatal(err)
	}

	ovsRow := vswitchd.OpenvSwitch{
		UUID: rootUUID,
	}
	mutateOps, err := ovs.Where(&ovsRow).Mutate(&ovsRow, model.Mutation{
		Field:   &ovsRow.Bridges,
		Mutator: "insert",
		Value:   []string{bridge.UUID},
	})
	if err != nil {
		log.Fatal(err)
	}

	operations := append(insertOp, mutateOps...)
	reply, err := ovs.Transact(ctx, operations...)
	if err != nil {
		log.Fatal(err)
	}
	if _, err := ovsdb.CheckOperationResults(reply, operations); err != nil {
		log.Fatal(err)
	}
	fmt.Println("Bridge Addition Successful : ", reply[0].UUID.GoUUID)
}

func processInput(ctx context.Context, ovs client.Client) {
	for {
		fmt.Printf("\n Enter a Bridge Name : ")
		var bridgeName string
		fmt.Scanf("%s", &bridgeName)
		if bridgeName == "" {
			continue
		}
		createBridge(ctx, ovs, bridgeName)
	}
}

func main() {
	ctx := context.Background()
	flag.Parse()
	quit = make(chan bool)
	update = make(chan model.Model)

	dbModel, err := model.NewDBModel("Open_vSwitch",
		map[string]model.Model{bridgeTable: &vswitchd.Bridge{}, ovsTable: &vswitchd.OpenvSwitch{}})
	if err != nil {
		log.Fatal("Unable to create DB model ", err)
	}

	ovs, err := client.NewOVSDBClient(dbModel, client.WithEndpoint(*connection))
	if err != nil {
		log.Fatal(err)
	}
	err = ovs.Connect(ctx)
	if err != nil {
		log.Fatal(err)
	}
	defer ovs.Disconnect()

	ovs.Cache().AddEventHandler(&cache.EventHandlerFuncs{
		AddFunc: func(table string, model model.Model) {
			if table == bridgeTable {
				update <- model
			}
		},
	})
	_, err = ovs.Monitor(ctx,
		ovs.NewTableMonitor(&vswitchd.OpenvSwitch{}),
		ovs.NewTableMonitor(&vswitchd.Bridge{}),
	)
	if err != nil {
		log.Fatal(err)
	}

	rootUUID = ovs.Cache().Table(ovsTable).Rows()[0]

	fmt.Println(`Silly game of stopping this app when a Bridge with name "stop" is monitored !`)
	go play(ctx, ovs)
	<-quit
}
