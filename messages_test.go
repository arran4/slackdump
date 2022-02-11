package slackdump

import (
	"context"
	"reflect"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/pkg/errors"
	"github.com/slack-go/slack"
	"github.com/stretchr/testify/assert"
	"golang.org/x/time/rate"
)

func Test_sortMessages(t *testing.T) {
	type args struct {
		msgs []Message
	}
	tests := []struct {
		name     string
		args     args
		wantMsgs []Message
	}{
		{
			"empty",
			args{[]Message{}},
			[]Message{},
		},
		{
			"sort ok",
			args{[]Message{
				{Message: slack.Message{Msg: slack.Msg{
					Timestamp: "1643425514",
				}}},
				{Message: slack.Message{Msg: slack.Msg{
					Timestamp: "1643425511",
				}}},
			}},
			[]Message{
				{Message: slack.Message{Msg: slack.Msg{
					Timestamp: "1643425511",
				}}},
				{Message: slack.Message{Msg: slack.Msg{
					Timestamp: "1643425514",
				}}},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sortMessages(tt.args.msgs)
			assert.Equal(t, tt.wantMsgs, tt.args.msgs)
		})
	}
}

func TestSlackDumper_convertMsgs(t *testing.T) {
	testMsg := slack.Message{Msg: slack.Msg{ClientMsgID: "a", Type: "x"}}
	testMsg2 := slack.Message{Msg: slack.Msg{ClientMsgID: "b", Type: "y"}}
	testMsg3 := slack.Message{Msg: slack.Msg{ClientMsgID: "c", Type: "z"}}
	type args struct {
		sm []slack.Message
	}
	tests := []struct {
		name string
		args args
		want []Message
	}{
		{
			"ok",
			args{[]slack.Message{
				testMsg,
				testMsg2,
				testMsg3,
			}},
			[]Message{
				{Message: testMsg},
				{Message: testMsg2},
				{Message: testMsg3},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sd := &SlackDumper{}
			if got := sd.convertMsgs(tt.args.sm); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("SlackDumper.convertMsgs() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSlackDumper_threadLeadMessage(t *testing.T) {
	type fields struct {
		Users     Users
		UserIndex map[string]*slack.User
		options   options
	}
	type args struct {
		ctx       context.Context
		l         *rate.Limiter
		channelID string
		threadTS  string
	}
	tests := []struct {
		name     string
		fields   fields
		args     args
		expectFn func(c *mockClienter)
		want     Message
		wantErr  bool
	}{
		{
			"all ok",
			fields{},
			args{context.Background(), newLimiter(noTier, 1, 0), "CHANNEL", "THREADTS"},
			func(c *mockClienter) {
				c.EXPECT().GetConversationHistoryContext(
					gomock.Any(),
					&slack.GetConversationHistoryParameters{
						ChannelID: "CHANNEL",
						Latest:    "THREADTS",
						Limit:     1,
						Inclusive: true,
					}).Return(
					&slack.GetConversationHistoryResponse{
						SlackResponse: slack.SlackResponse{Ok: true},
						Messages: []slack.Message{
							{Msg: slack.Msg{ClientMsgID: "X", Type: "Y"}},
						},
					},
					nil)
			},
			Message{Message: slack.Message{Msg: slack.Msg{ClientMsgID: "X", Type: "Y"}}},
			false,
		},
		{
			"resp not ok",
			fields{},
			args{context.Background(), newLimiter(noTier, 1, 0), "CHANNEL", "THREADTS"},
			func(c *mockClienter) {
				c.EXPECT().GetConversationHistoryContext(
					gomock.Any(),
					gomock.Any(),
				).Return(
					&slack.GetConversationHistoryResponse{
						SlackResponse: slack.SlackResponse{Ok: false},
					},
					nil)
			},
			Message{},
			true,
		},
		{
			"sudden bleep bloop error",
			fields{},
			args{context.Background(), newLimiter(noTier, 1, 0), "CHANNEL", "THREADTS"},
			func(c *mockClienter) {
				c.EXPECT().GetConversationHistoryContext(
					gomock.Any(),
					gomock.Any(),
				).Return(
					nil,
					errors.New("bleep bloop gtfo"))
			},
			Message{},
			true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mc := newmockClienter(ctrl)

			tt.expectFn(mc)

			sd := &SlackDumper{
				client:    mc,
				Users:     tt.fields.Users,
				UserIndex: tt.fields.UserIndex,
				options:   tt.fields.options,
			}
			got, err := sd.threadLeadMessage(tt.args.ctx, tt.args.l, tt.args.channelID, tt.args.threadTS)
			if (err != nil) != tt.wantErr {
				t.Errorf("SlackDumper.threadLeadMessage() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("SlackDumper.threadLeadMessage() = %v, want %v", got, tt.want)
			}
		})
	}
}
