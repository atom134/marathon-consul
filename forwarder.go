package main

import (
	"github.com/hashicorp/consul/api"
)

type Putter interface {
	Put(*api.KVPair, *api.WriteOptions) (*api.WriteMeta, error)
}

type Forwarder struct {
	kv           Putter
	parallelism  int
	WriteOptions *api.WriteOptions
}

func NewForwarder(config *api.Config, options *api.WriteOptions) (*Forwarder, error) {
	client, err := api.NewClient(config)
	if err != nil {
		return nil, err
	}

	// TODO: make parallelism tunable
	return &Forwarder{client.KV(), 8, options}, nil
}

type WorkMessage struct {
	ID int
	KV *api.KVPair
}

type ErrorMessage struct {
	ID    int
	Error error
}

func (f *Forwarder) MultiPut(kvs []*api.KVPair) []error {
	kvChan := make(chan WorkMessage, len(kvs))
	for i, kv := range kvs {
		kvChan <- WorkMessage{i, kv}
	}

	errChan := make(chan ErrorMessage, len(kvs))
	for i := 0; i <= f.parallelism; i++ {
		go f.worker(kvChan, errChan)
	}

	// collect errors without regard for order
	errors := map[int]error{}
	for range kvs {
		message := <-errChan
		errors[message.ID] = message.Error
	}

	// transform the map of ID -> error into the original order
	out := []error{}
	for i := range kvs {
		out = append(out, errors[i])
	}
	return out
}

func (f *Forwarder) worker(in chan WorkMessage, out chan ErrorMessage) {
	for {
		select {
		case message := <-in:
			_, err := f.kv.Put(message.KV, f.WriteOptions)
			out <- ErrorMessage{message.ID, err}
		default:
			return
		}
	}
}