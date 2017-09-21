package discovery

import (
	"errors"
	"sync"
	"time"

	"github.com/agilab/telegraf_api/internal/logger"

	"google.golang.org/grpc/naming"

	"golang.org/x/net/context"

	etcd "github.com/coreos/etcd/clientv3"
	etcdnaming "github.com/coreos/etcd/clientv3/naming"
)

const closeContextTimeout = 5 * time.Second

//ErrDiscoveryServiceKeepAliveInvalidCall KeepAlive不能被调用两次
var ErrDiscoveryServiceKeepAliveInvalidCall = errors.New("discovery: KeepAlive cant be called twice for one Service")

//ErrDiscoveryServiceClosed Service is closed
var ErrDiscoveryServiceClosed = errors.New("discovery: Service is closed")

//Service etcd discoverer
type Service struct {
	mu sync.Mutex

	ctx    context.Context
	cancel context.CancelFunc

	service string
	address *Address
	ttl     int64
	logger  *logger.Logger

	client         *etcd.Client
	leaseGrantResp *etcd.LeaseGrantResponse
	resolver       *etcdnaming.GRPCResolver

	keeping bool
	done    bool
}

//Address service addr and metadata
type Address struct {
	Addr     string
	Metadata string
}

//NewService new service for registering into etcd
func NewService(client *etcd.Client, service string, address *Address, ttl int64, logger *logger.Logger) *Service {
	if ttl < 3 {
		ttl = 3
	}
	d := &Service{
		client:  client,
		service: service,
		address: address,
		ttl:     ttl,
		logger:  logger,
	}
	d.ctx, d.cancel = context.WithCancel(client.Ctx())
	return d
}

//Close close
func (d *Service) Close() error {
	//取消当前所有etcd的rpc请求
	d.cancel()

	//此时再拿锁，KeepAlive会因为上述的cancel将锁解除
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.done {
		return ErrDiscoveryServiceClosed
	}
	d.done = true

	if d.resolver != nil {
		ctx, cancel := context.WithTimeout(d.resolver.Client.Ctx(), time.Duration(d.ttl)*time.Second)
		if err := d.resolver.Update(ctx,
			d.service,
			naming.Update{
				Op:   naming.Delete,
				Addr: d.address.Addr}); err != nil && err != context.Canceled {
			d.logger.Errorf("[discovery] delete service error - %s", err)
		} else {
			d.logger.Infof("[discovery] unregister service(%s) from etcd", d.service+"/"+d.address.Addr)
		}
		cancel()
		d.resolver = nil
	}

	if d.client != nil {
		if d.leaseGrantResp != nil {
			ctx, cancel := context.WithTimeout(d.client.Ctx(), time.Duration(d.ttl)*time.Second)
			if _, err := d.client.Revoke(ctx, d.leaseGrantResp.ID); err != nil && err != context.Canceled {
				d.logger.Errorf("[discovery] revoke lease error - %s", err)
			}
			cancel()
			d.leaseGrantResp = nil
		}
	}

	return nil
}

func (d *Service) register() error {
	cli := d.client

	ctx, _ := context.WithCancel(d.ctx)
	resp, err := cli.Grant(ctx, d.ttl)
	if err != nil {
		return err
	}
	d.leaseGrantResp = resp

	r := &etcdnaming.GRPCResolver{Client: cli}
	d.resolver = r

	opts := etcd.WithLease(resp.ID)
	ctx, _ = context.WithCancel(d.ctx)
	if err := r.Update(ctx,
		d.service,
		naming.Update{
			Op:       naming.Add,
			Addr:     d.address.Addr,
			Metadata: d.address.Metadata},
		opts); err != nil {
		return err
	}

	d.logger.Infof("[discovery] register service(%s) into etcd", d.service+"/"+d.address.Addr)

	return nil
}

//KeepAlive keepalive
func (d *Service) KeepAlive() error {
	d.mu.Lock()
	if d.done {
		d.mu.Unlock()
		return ErrDiscoveryServiceClosed
	}

	if d.keeping || d.client == nil {
		d.mu.Unlock()
		return ErrDiscoveryServiceKeepAliveInvalidCall
	}

	defer func() {
		d.keeping = false
	}()
	d.keeping = true

	//KeepAlive 实现
	err := d.register()
	if err != nil {
		d.mu.Unlock()
		return err
	}

	ctx, _ := context.WithCancel(d.ctx)
	ch, err := d.client.KeepAlive(ctx, d.leaseGrantResp.ID)
	if err != nil {
		d.mu.Unlock()
		return err
	}

	d.mu.Unlock()

	for {
		select {
		case <-d.ctx.Done():
			return d.ctx.Err()
		case resp := <-ch:
			if resp == nil {
				d.mu.Lock()

				if d.done {
					d.mu.Unlock()
					return ErrDiscoveryServiceClosed
				}

				err = d.register()
				if err != nil {
					d.mu.Unlock()
					return err
				}

				ctx, _ = context.WithCancel(d.ctx)
				ch, err = d.client.KeepAlive(ctx, d.leaseGrantResp.ID)
				if err != nil {
					d.mu.Unlock()
					return err
				}
				d.mu.Unlock()
			}
			d.logger.Debugf("[discovery] service keepalive")
		}
	}

	//KeepAliveOnce 实现

	// err = d.register()
	// if err != nil {
	// 	d.mu.Unlock()
	// 	return err
	// }

	// d.mu.Unlock()

	// ticker := time.NewTicker(time.Duration(d.ttl/3) * time.Second)
	// defer ticker.Stop()

	// for {
	// 	select {
	// 	case <-d.ctx.Done():
	// 		return d.ctx.Err()
	// 	case <-ticker.C:
	// 		var err error

	// 		d.mu.Lock()

	//if d.done {
	//	d.mu.Unlock()
	//	return ErrDiscoveryClosed
	//}
	// 		if d.leaseGrantResp != nil {
	// 			ctx, _ = context.WithCancel(d.ctx)
	// 			_, err = d.client.KeepAliveOnce(ctx, d.leaseGrantResp.ID)
	// 			if err == nil {
	// 				d.logger.Debugf("[discovery] service keepalive %v", d.leaseGrantResp.ID)
	// 			} else if err == rpctypes.ErrLeaseNotFound {
	// 				err = d.register()
	// 			}
	// 		} else {
	// 			err = d.register()
	// 		}

	// 		d.mu.Unlock()

	// 		if err != nil {
	// 			return err
	// 		}
	// 	}
	// }
}
