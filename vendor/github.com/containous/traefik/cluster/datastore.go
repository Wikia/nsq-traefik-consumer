package cluster

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/cenk/backoff"
	"github.com/containous/staert"
	"github.com/containous/traefik/job"
	"github.com/containous/traefik/log"
	"github.com/containous/traefik/safe"
	"github.com/docker/libkv/store"
	"github.com/satori/go.uuid"
)

// Metadata stores Object plus metadata
type Metadata struct {
	object Object
	Object []byte
	Lock   string
}

// NewMetadata returns new Metadata
func NewMetadata(object Object) *Metadata {
	return &Metadata{object: object}
}

// Marshall marshalls object
func (m *Metadata) Marshall() error {
	var err error
	m.Object, err = json.Marshal(m.object)
	return err
}

func (m *Metadata) unmarshall() error {
	if len(m.Object) == 0 {
		return nil
	}
	return json.Unmarshal(m.Object, m.object)
}

// Listener is called when Object has been changed in KV store
type Listener func(Object) error

var _ Store = (*Datastore)(nil)

// Datastore holds a struct synced in a KV store
type Datastore struct {
	kv        staert.KvSource
	ctx       context.Context
	localLock *sync.RWMutex
	meta      *Metadata
	lockKey   string
	listener  Listener
}

// NewDataStore creates a Datastore
func NewDataStore(ctx context.Context, kvSource staert.KvSource, object Object, listener Listener) (*Datastore, error) {
	datastore := Datastore{
		kv:        kvSource,
		ctx:       ctx,
		meta:      &Metadata{object: object},
		lockKey:   kvSource.Prefix + "/lock",
		localLock: &sync.RWMutex{},
		listener:  listener,
	}
	err := datastore.watchChanges()
	if err != nil {
		return nil, err
	}
	return &datastore, nil
}

func (d *Datastore) watchChanges() error {
	stopCh := make(chan struct{})
	kvCh, err := d.kv.Watch(d.lockKey, stopCh)
	if err != nil {
		return err
	}
	safe.Go(func() {
		ctx, cancel := context.WithCancel(d.ctx)
		operation := func() error {
			for {
				select {
				case <-ctx.Done():
					stopCh <- struct{}{}
					return nil
				case _, ok := <-kvCh:
					if !ok {
						cancel()
						return err
					}
					err = d.reload()
					if err != nil {
						return err
					}
					if d.listener != nil {
						err := d.listener(d.meta.object)
						if err != nil {
							log.Errorf("Error calling datastore listener: %s", err)
						}
					}
				}
			}
		}
		notify := func(err error, time time.Duration) {
			log.Errorf("Error in watch datastore: %+v, retrying in %s", err, time)
		}
		err := backoff.RetryNotify(safe.OperationWithRecover(operation), job.NewBackOff(backoff.NewExponentialBackOff()), notify)
		if err != nil {
			log.Errorf("Error in watch datastore: %v", err)
		}
	})
	return nil
}

func (d *Datastore) reload() error {
	log.Debug("Datastore reload")
	d.localLock.Lock()
	err := d.kv.LoadConfig(d.meta)
	if err != nil {
		d.localLock.Unlock()
		return err
	}
	err = d.meta.unmarshall()
	if err != nil {
		d.localLock.Unlock()
		return err
	}
	d.localLock.Unlock()
	return nil
}

// Begin creates a transaction with the KV store.
func (d *Datastore) Begin() (Transaction, Object, error) {
	id := uuid.NewV4().String()
	log.Debugf("Transaction %s begins", id)
	remoteLock, err := d.kv.NewLock(d.lockKey, &store.LockOptions{TTL: 20 * time.Second, Value: []byte(id)})
	if err != nil {
		return nil, nil, err
	}
	stopCh := make(chan struct{})
	ctx, cancel := context.WithCancel(d.ctx)
	var errLock error
	go func() {
		_, errLock = remoteLock.Lock(stopCh)
		cancel()
	}()
	select {
	case <-ctx.Done():
		if errLock != nil {
			return nil, nil, errLock
		}
	case <-d.ctx.Done():
		stopCh <- struct{}{}
		return nil, nil, d.ctx.Err()
	}

	// we got the lock! Now make sure we are synced with KV store
	operation := func() error {
		meta := d.get()
		if meta.Lock != id {
			return fmt.Errorf("Object lock value: expected %s, got %s", id, meta.Lock)
		}
		return nil
	}
	notify := func(err error, time time.Duration) {
		log.Errorf("Datastore sync error: %v, retrying in %s", err, time)
		err = d.reload()
		if err != nil {
			log.Errorf("Error reloading: %+v", err)
		}
	}
	ebo := backoff.NewExponentialBackOff()
	ebo.MaxElapsedTime = 60 * time.Second
	err = backoff.RetryNotify(safe.OperationWithRecover(operation), ebo, notify)
	if err != nil {
		return nil, nil, fmt.Errorf("Datastore cannot sync: %v", err)
	}

	// we synced with KV store, we can now return Setter
	return &datastoreTransaction{
		Datastore:  d,
		remoteLock: remoteLock,
		id:         id,
	}, d.meta.object, nil
}

func (d *Datastore) get() *Metadata {
	d.localLock.RLock()
	defer d.localLock.RUnlock()
	return d.meta
}

// Load load atomically a struct from the KV store
func (d *Datastore) Load() (Object, error) {
	d.localLock.Lock()
	defer d.localLock.Unlock()
	err := d.kv.LoadConfig(d.meta)
	if err != nil {
		return nil, err
	}
	err = d.meta.unmarshall()
	if err != nil {
		return nil, err
	}
	return d.meta.object, nil
}

// Get atomically a struct from the KV store
func (d *Datastore) Get() Object {
	d.localLock.RLock()
	defer d.localLock.RUnlock()
	return d.meta.object
}

var _ Transaction = (*datastoreTransaction)(nil)

type datastoreTransaction struct {
	*Datastore
	remoteLock store.Locker
	dirty      bool
	id         string
}

// Commit allows to set an object in the KV store
func (s *datastoreTransaction) Commit(object Object) error {
	s.localLock.Lock()
	defer s.localLock.Unlock()
	if s.dirty {
		return fmt.Errorf("Transaction already used, please begin a new one")
	}
	s.Datastore.meta.object = object
	err := s.Datastore.meta.Marshall()
	if err != nil {
		return fmt.Errorf("Marshall error: %s", err)
	}
	err = s.kv.StoreConfig(s.Datastore.meta)
	if err != nil {
		return fmt.Errorf("StoreConfig error: %s", err)
	}

	err = s.remoteLock.Unlock()
	if err != nil {
		return fmt.Errorf("Unlock error: %s", err)
	}

	s.dirty = true
	log.Debugf("Transaction committed %s", s.id)
	return nil
}
