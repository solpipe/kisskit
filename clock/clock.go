package clock

import (
	"context"
	"errors"
	"log"
	"time"

	sgorpc "github.com/SolmateDev/solana-go/rpc"
	sgows "github.com/SolmateDev/solana-go/rpc/ws"
	dssub "github.com/solpipe/kiss-kit/sub"
)

type Clock struct {
	ctx       context.Context
	internalC chan<- func(*internal)
	slotReqC  chan dssub.ResponseChannel[Update]
}

func Create(
	ctx context.Context,
	rpcClient *sgorpc.Client,
	wsClient *sgows.Client,
) (c Clock, err error) {

	sub, err := wsClient.SlotSubscribe()
	if err != nil {
		return
	}
	internalC := make(chan func(*internal))
	var slot uint64
	{
		slot, err = rpcClient.GetSlot(ctx, sgorpc.CommitmentFinalized)
		if err != nil {
			return
		}
	}

	slotSubHome := dssub.CreateSubHome[Update]()

	go loopInternal(ctx, internalC, sub, slot, slotSubHome)
	c = Clock{
		ctx:       ctx,
		internalC: internalC,
		slotReqC:  slotSubHome.ReqC,
	}

	return c, nil
}

type internal struct {
	ctx              context.Context
	closeSignalCList []chan<- error
	slot             uint64
	lastSlotTime     time.Time
	slotSubHome      *dssub.SubHome[Update]
}

func loopInternal(
	ctx context.Context,
	internalC <-chan func(*internal),
	sub *sgows.SlotSubscription,
	slot uint64,
	slotSubHome *dssub.SubHome[Update],
) {
	var err error
	doneC := ctx.Done()
	in := new(internal)
	in.ctx = ctx
	in.closeSignalCList = make([]chan<- error, 0)
	in.slot = slot
	in.slotSubHome = slotSubHome
	defer slotSubHome.Close()

	defer sub.Unsubscribe()
	streamC := sub.RecvStream()
	errorC := sub.CloseSignal()
	in.lastSlotTime = time.Now()
out:
	for {
		select {
		case <-doneC:
			break out
		case err = <-errorC:
			break out
		case req := <-internalC:
			req(in)
		case x := <-streamC:
			d, ok := x.(*sgows.SlotResult)
			if !ok {
				err = errors.New("bad slot result")
				break out
			}
			in.slot = d.Slot
			n := time.Now()
			in.slotSubHome.Broadcast(Update{
				Slot:              in.slot,
				TimeSinceLastSlot: n.Sub(in.lastSlotTime),
			})
			in.lastSlotTime = n
		case id := <-slotSubHome.DeleteC:
			slotSubHome.Delete(id)
		case r := <-slotSubHome.ReqC:
			slotSubHome.Receive(r)
		}
	}
	in.finish(err)
}

func (in *internal) finish(err error) {
	log.Print(err)
	for i := 0; i < len(in.closeSignalCList); i++ {
		in.closeSignalCList[i] <- err
	}
}

func (c Clock) CloseSignal() <-chan error {
	signalC := make(chan error, 1)
	doneC := c.ctx.Done()
	select {
	case <-doneC:
		signalC <- errors.New("canceled")
	case c.internalC <- func(in *internal) {
		in.closeSignalCList = append(in.closeSignalCList, signalC)
	}:
	}
	return signalC
}

type Update struct {
	Slot              uint64
	TimeSinceLastSlot time.Duration
}

func (c Clock) OnSlot() dssub.Subscription[Update] {
	return dssub.SubscriptionRequest(c.slotReqC, func(x Update) bool { return true })
}

func (c Clock) Slot() (Update, error) {
	err := c.ctx.Err()
	if err != nil {
		return Update{}, err
	}
	doneC := c.ctx.Done()
	ansC := make(chan Update, 1)
	n := time.Now()
	select {
	case <-doneC:
		return Update{}, errors.New("canceled")
	case c.internalC <- func(in *internal) {
		ansC <- Update{
			Slot:              in.slot,
			TimeSinceLastSlot: n.Sub(in.lastSlotTime),
		}
	}:
	}
	return <-ansC, nil
}
