package ebpf

import (
	"bytes"
	"fmt"
	"log"

	bpf "github.com/iovisor/gobpf/bcc"

	"github.com/mehrdadrad/tcpdog/config"
)

// BPF represents eBPF procedures.
type BPF struct {
	m        *bpf.Module
	perfMaps []*bpf.PerfMap
}

type TP struct {
	Name    string
	Index   int
	OutChan chan *bytes.Buffer
	Workers int
	INet    []int
	Fields  []string
}

// New generates and loads the bpf program.
func New(conf *config.Config) *BPF {
	code, err := GetBPFCode(conf)
	if err != nil {
		log.Fatal(err)
	}

	m := bpf.NewModule(code, []string{})

	return &BPF{m: m}
}

// Start ...
func (b *BPF) Start(tp TP) {
	trace, err := b.m.LoadTracepoint(fmt.Sprintf("sk_trace%d", tp.Index))
	if err != nil {
		log.Fatal(err)
	}

	b.m.AttachTracepoint(tp.Name, trace)

	for _, version := range tp.INet {
		table := bpf.NewTable(b.m.TableId(fmt.Sprintf("ipv%d_events%d", version, tp.Index)), b.m)
		ch := make(chan []byte, 1000)
		perfMap, err := bpf.InitPerfMap(table, ch, nil)
		if err != nil {
			log.Fatal(err)
		}

		go func(version int) {
			d := newDecoder((version == 4))
			for {
				data := <-ch
				//log.Printf("%#v\n", data)
				buf := new(bytes.Buffer)
				d.decode(data, tp.Fields, buf)
				tp.OutChan <- buf
			}
		}(version)

		perfMap.Start()
		b.perfMaps = append(b.perfMaps, perfMap)
	}
}

// Close ...
func (b *BPF) Close() {
	for _, perfMap := range b.perfMaps {
		perfMap.Stop()
	}
	b.m.Close()
}
