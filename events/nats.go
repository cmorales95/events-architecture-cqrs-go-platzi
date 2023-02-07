package events

import (
	"bytes"
	"context"
	"encoding/gob"

	"go-cqrs/models"

	"github.com/nats-io/nats.go"
)

type NatsEventStore struct {
	conn            *nats.Conn
	feedCreatedSub  *nats.Subscription
	feedCreatedChan chan CreatedFeedMessage
}

func NewNats(url string) (*NatsEventStore, error) {
	conn, err := nats.Connect(url)
	if err != nil {
		return nil, err
	}
	return &NatsEventStore{conn: conn}, nil
}

func (n *NatsEventStore) PublishCreatedFeed(ctx context.Context, feed *models.Feed) error {
	msg := CreatedFeedMessage{
		ID:          feed.ID,
		Title:       feed.Title,
		Description: feed.Description,
		CreatedAt:   feed.CreatedAt,
	}

	data, err := n.encodeMessage(msg)

	if err != nil {
		return err
	}

	return n.conn.Publish(msg.Type(), data)
}

func (n *NatsEventStore) SubscribeCreatedFeed(ctx context.Context) (<-chan CreatedFeedMessage, error) {
	var m CreatedFeedMessage

	n.feedCreatedChan = make(chan CreatedFeedMessage, 64)
	ch := make(chan *nats.Msg, 64)

	var err error
	n.feedCreatedSub, err = n.conn.ChanSubscribe(m.Type(), ch)
	if err != nil {
		return nil, err
	}

	go func() {
		for {
			select {
			case msg := <-ch:
				n.decodeMessage(msg.Data, &m)
				n.feedCreatedChan <- m
			}
		}
	}()

	return (<-chan CreatedFeedMessage)(n.feedCreatedChan), nil
}

func (n *NatsEventStore) OnCreateFeed(f func(message CreatedFeedMessage)) (err error) {
	var feedMessage CreatedFeedMessage

	n.feedCreatedSub, err = n.conn.Subscribe(feedMessage.Type(), func(msg *nats.Msg) {
		n.decodeMessage(msg.Data, &feedMessage)
		f(feedMessage)
	})

	return
}

func (n *NatsEventStore) Close() {
	if n.conn != nil {
		n.conn.Close()
	}

	if n.feedCreatedSub != nil {
		n.feedCreatedSub.Unsubscribe()
	}

	close(n.feedCreatedChan)
}

func (n *NatsEventStore) encodeMessage(m Message) ([]byte, error) {
	var b bytes.Buffer

	err := gob.NewEncoder(&b).Encode(m)
	if err != nil {
		return nil, err
	}

	return b.Bytes(), nil
}

func (n *NatsEventStore) decodeMessage(data []byte, m interface{}) error {
	var b bytes.Buffer
	b.Write(data)

	return gob.NewDecoder(&b).Decode(m)
}