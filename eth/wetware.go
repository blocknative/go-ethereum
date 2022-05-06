package eth

import (
	"context"
	"fmt"
	"math/big"
	"net"
	"time"

	"capnproto.org/go/capnp/v3"
	"github.com/blocknative/bn/pkg/ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/eth/filters"
	"github.com/ethereum/go-ethereum/internal/ethapi"
	"github.com/ethereum/go-ethereum/log"
	"github.com/libp2p/go-libp2p"
	libp2pquic "github.com/libp2p/go-libp2p-quic-transport"
	"github.com/multiformats/go-multiaddr"
	manet "github.com/multiformats/go-multiaddr/net"
	"github.com/wetware/casm/pkg/boot/crawl"
	"github.com/wetware/ww/pkg/client"
	"github.com/wetware/ww/pkg/vat"
	"go.uber.org/multierr"
	"golang.org/x/sync/errgroup"
)

// wetware side-bus, which streams capnp-encoded events over pubsub.
type wetware struct {
	Backend  ethapi.Backend
	ChainID  *big.Int
	NS, Boot string

	conn net.PacketConn // bootstrap
	n    *client.Node
}

// Start is called after all services have been constructed and the networking
// layer was also initialized to spawn any goroutines required by the service.
func (ww *wetware) Start() (err error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()

	ww.n, err = ww.dial(ctx)
	if err == nil {
		go ww.serve()
	}

	return
}

func (ww *wetware) dial(ctx context.Context) (*client.Node, error) {
	if ww.NS == "" {
		ww.NS = "bn"
	}

	if ww.Boot == "" {
		ww.Boot = "/ip4/10.0.1.0/udp/8822/cidr/24"
	}

	h, err := libp2p.New(
		libp2p.NoTransports,
		libp2p.NoListenAddrs,
		libp2p.Transport(libp2pquic.NewTransport))
	if err != nil {
		return nil, err
	}

	s, err := ww.dialBootstrap(ctx)
	if err != nil {
		return nil, err
	}

	return client.Dialer{
		Vat: vat.Network{
			NS:   ww.NS,
			Host: h,
		},
		Boot: crawl.New(h, ww.conn, s),
	}.Dial(ctx)
}

func (ww *wetware) dialBootstrap(ctx context.Context) (crawl.Strategy, error) {
	maddr, err := multiaddr.NewMultiaddr(ww.Boot)
	if err != nil {
		return nil, err
	}

	s, err := crawl.ParseCIDR(maddr)
	if err != nil {
		return nil, err
	}

	network, _, err := manet.DialArgs(maddr)
	if err != nil {
		return nil, err
	}

	ww.conn, err = net.ListenPacket(network, ":0")
	return s, err
}

// Stop terminates all goroutines belonging to the service, blocking until they
// are all terminated.
func (ww *wetware) Stop() (err error) {
	return multierr.Combine(
		ww.n.Close(),
		ww.conn.Close(),
		ww.n.Host().Close())
}

func (ww *wetware) serve() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	p := publisher{Topic: ww.topic(ctx)} // FIXME:  determine network dynamically
	defer p.Release()

	events := filters.NewEventSystem(ww.Backend, true)

	g, ctx := errgroup.WithContext(ctx)
	g.Go(ww.publishPendingTx(ctx, events, p))

	if err := g.Wait(); err != nil && err != context.Canceled {
		log.Error("wetware: %s", err)
	}
}

func (ww *wetware) topic(ctx context.Context) client.Topic {
	var network string
	switch ww.ChainID.Uint64() {
	case 1:
		network = "main"
	case 3:
		network = "ropsten"
	case 4:
		network = "rinkeby"
	case 5:
		network = "goerli"
	case 1337802:
		network = "kiln"
	default:
		panic(fmt.Sprintf("unrecognized network ID %d", ww.ChainID.Uint64()))
	}

	return ww.n.Join(ctx, fmt.Sprintf("bn.monitor.ethereum.%s", network))
}

func (ww *wetware) publishPendingTx(ctx context.Context, events *filters.EventSystem, p publisher) func() error {
	return func() error {
		hs := make(chan []common.Hash, 16)
		defer close(hs)

		sub := events.SubscribePendingTxs(hs)
		defer sub.Unsubscribe()

		var tx ethereum.Transaction
		for {
			select {
			case batch := <-hs:
				for _, h := range batch {
					if err := tx.FromEthTx(ww.Backend.GetPoolTransaction(h)); err != nil {
						return err
					}

					if err := p.Publish(ctx, tx.Message()); err != nil {
						return err
					}
				}

			case err := <-sub.Err():
				return err

			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}
}

type publisher struct{ client.Topic }

func (e publisher) Publish(ctx context.Context, m *capnp.Message) error {
	b, err := m.MarshalPacked()
	if err != nil {
		return err
	}

	return e.Topic.Publish(ctx, b)
}
