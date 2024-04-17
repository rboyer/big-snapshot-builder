package main

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"sync"
	"time"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-uuid"
)

const (
	// programName = "big-snapshot-builder"
	programName = "bsb"

	// kvBlobSizeBytes = 400 * 1024
	kvBlobSizeBytes = 400 //* 1024
	kvNumBlobs      = 40_000
	// kvNumBlobs      = 7000
)

func main() {
	log.SetOutput(ioutil.Discard)

	// Create logger
	logger := hclog.New(&hclog.LoggerOptions{
		Name:       programName,
		Level:      hclog.Debug,
		Output:     os.Stderr,
		JSONFormat: false,
	})

	if len(os.Args) != 2 {
		logger.Error("missing required mode argument <catalog|kv>")
		os.Exit(1)
		return
	}

	if err := run(logger, os.Args[1]); err != nil {
		logger.Error(err.Error())
		os.Exit(1)
	}
}

func run(logger hclog.Logger, mode string) error {
	cfg := api.DefaultConfigWithLogger(logger)

	client, err := api.NewClient(cfg)
	if err != nil {
		return err
	}

	switch mode {
	case "catalog":
		if err := populateCatalog(logger, client); err != nil {
			return err
		}
	case "kv":
		if err := populateKV(logger, client); err != nil {
			return err
		}
	case "kvclean":
		if err := cleanKV(logger, client); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unknown mode %q", mode)
	}

	return nil
}

const (
	numWorker        = 1000
	numNodes         = 300_000
	servicesPerNode  = 10
	checksPerService = 3

	nodeMetaBytes  = 1 * 1024
	svcMetaBytes   = 500
	checkMetaBytes = 1 * 1024
)

func populateCatalog(logger hclog.Logger, client *api.Client) error {
	var (
		wg        sync.WaitGroup
		workQueue = make(chan int)
	)

	for wrk := 0; wrk < numWorker; wrk++ {
		loggerW := logger.With("worker", fmt.Sprintf("%d", wrk))

		wg.Add(1)
		go func(wrk int) {
			defer wg.Done()

			logger := loggerW

			for nodeIdx := range workQueue {
				start := time.Now()
				err := populateNode(logger, client, nodeIdx)
				logger.Info("populateNode time", "dur", time.Since(start))
				if err != nil {
					logger.Error("error populating node", "error", err)
				}
			}
		}(wrk)
	}

	for nodeIdx := 0; nodeIdx < numNodes; nodeIdx++ {
		if nodeIdx%100 == 0 {
			logger.Info("node progress", "current", nodeIdx, "total", numNodes)
		}
		workQueue <- nodeIdx
	}
	close(workQueue)

	wg.Wait()

	return nil
}

func populateNode(logger hclog.Logger, client *api.Client, nodeIdx int) error {
	catalog := client.Catalog()

	nodeName := fmt.Sprintf("client-%05d", nodeIdx)

	reg := &api.CatalogRegistration{
		ID:      uniqueID(),
		Node:    nodeName,
		Address: makeIP(nodeIdx),
		// NodeMeta: map[string]string{
		// 	"blob": newHexBlob(nodeMetaBytes),
		// },
		// Service         *AgentService
		// Check           *AgentCheck
		// Checks          HealthChecks
	}

	// start := time.Now()
	// if _, err := catalog.Register(reg, nil); err != nil {
	// 	return err
	// }
	// logger.Info("node register", "dur", time.Since(start))

	for svcIdx := 0; svcIdx < servicesPerNode; svcIdx++ {
		svcName := fmt.Sprintf("app-%05d", svcIdx)
		svc := &api.AgentService{
			ID:      svcName,
			Service: svcName,
			Port:    8080 + svcIdx,
			// Meta: map[string]string{
			// 	"blob": newHexBlob(svcMetaBytes),
			// },
		}
		reg.Service = svc

		// start = time.Now()
		if _, err := catalog.Register(reg, nil); err != nil {
			return err
		}
		// logger.Info("service register", "dur", time.Since(start))

		reg.Service = nil

		for chkIdx := 0; chkIdx < checksPerService; chkIdx++ {
			chkName := fmt.Sprintf("chk-%d", chkIdx)
			chk := &api.HealthCheck{
				Node:        nodeName,
				CheckID:     chkName,
				Name:        chkName,
				Status:      "passing",
				ServiceID:   svcName,
				ServiceName: svcName,
				// Output:      newHexBlob(checkMetaBytes),
				// Status      string
				// Notes       string
				// Output      string
				// ServiceTags []string
				// Type        string
				// Definition HealthCheckDefinition
			}

			if chkIdx == 0 {
				chk.ServiceName = ""
				chk.ServiceID = ""
			}

			reg.Checks = append(reg.Checks, chk)
		}

		// start = time.Now()
		if _, err := catalog.Register(reg, nil); err != nil {
			return err
		}
		// logger.Info("check register", "dur", time.Since(start))
	}
	return nil
}

func makeIP(n int) string {
	a := n / 128
	b := (n - a*128)

	if b >= 128 {
		panic("n is too big")
	}

	return fmt.Sprintf("10.10.%d.%d", a, b)
}

func populateKV(logger hclog.Logger, client *api.Client) error {
	kv := client.KV()

	for i := 0; i < kvNumBlobs; i++ {
		if i%100 == 0 {
			logger.Info("kv progress", "current", i, "total", kvNumBlobs)
		}
		key := fmt.Sprintf("key-%d", i)
		val := newBlob(kvBlobSizeBytes)

		_, err := kv.Put(&api.KVPair{
			Key:   key,
			Value: val,
		}, nil)
		if err != nil {
			return err
		}
	}

	return nil
}

func cleanKV(logger hclog.Logger, client *api.Client) error {
	kv := client.KV()

	_, err := kv.DeleteTree("/", nil)
	return err
}

func newHexBlob(size int) string {
	b := newBlob(size / 2)
	return hex.EncodeToString(b)
}

func newBlob(size int) []byte {
	b := make([]byte, size)
	_, err := io.ReadFull(rand.Reader, b)
	if err != nil {
		panic(err)
	}
	return b
}

func uniqueID() string {
	id, err := uuid.GenerateUUID()
	if err != nil {
		panic(err)
	}
	return id
}
