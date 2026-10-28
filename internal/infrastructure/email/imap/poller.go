// Package imap polls each context's IMAP INBOX and stores new messages
// in the per-context email inbox.
package imap

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"strings"
	"time"

	"github.com/emersion/go-imap"
	imapclient "github.com/emersion/go-imap/client"
	"github.com/rs/zerolog/log"

	appcontext "github.com/h2hsecure/automata/internal/domain/context"
	"github.com/h2hsecure/automata/internal/domain/email"
	mailsmtp "github.com/h2hsecure/automata/internal/infrastructure/email/smtp"
)

const (
	pollInterval = 60 * time.Second
	maxMessages  = 500
	firstRunCap  = 50
	imapPort     = "993"
	textSection  = imap.FetchItem("BODY.PEEK[TEXT]")
)

// Lister lists contexts; satisfied by context.Repository.
type Lister interface {
	List(offset, limit int) ([]*appcontext.Context, error)
}

// Poller fetches new INBOX messages for every context that has SMTP
// credentials configured.
type Poller struct {
	inboxRepo email.InboxRepository
	contexts  Lister
	configFor func(ctxID string) mailsmtp.Config
}

// NewPoller creates a new inbox poller.
func NewPoller(inboxRepo email.InboxRepository, contexts Lister, configFor func(ctxID string) mailsmtp.Config) *Poller {
	return &Poller{inboxRepo: inboxRepo, contexts: contexts, configFor: configFor}
}

// Start runs the poller until ctx is cancelled. The first run is immediate.
func (p *Poller) Start(ctx context.Context) {
	p.run(ctx)
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p.run(ctx)
		}
	}
}

func (p *Poller) run(ctx context.Context) {
	contexts, err := p.contexts.List(0, 1000)
	if err != nil {
		log.Warn().Err(err).Msg("inbox poll: list contexts")
		return
	}
	for _, c := range contexts {
		if ctx.Err() != nil {
			return
		}
		p.pollContext(ctx, c.ID)
	}
}

func (p *Poller) pollContext(ctx context.Context, ctxID string) {
	cfg := p.configFor(ctxID)
	if cfg.Host == "" || cfg.Username == "" {
		return // no IMAP credentials for this context
	}

	last, err := p.inboxRepo.MaxUID(ctxID)
	if err != nil {
		log.Warn().Err(err).Str("context_id", ctxID).Msg("inbox poll: max uid")
		return
	}

	c, err := imapclient.DialTLS(net.JoinHostPort(cfg.Host, imapPort),
		&tls.Config{ServerName: cfg.Host})
	if err != nil {
		log.Warn().Err(err).Str("context_id", ctxID).Msg("inbox poll: connect")
		return
	}
	defer c.Logout()
	// Bound IMAP command I/O so a stalled server cannot wedge the poller loop.
	c.Timeout = 30 * time.Second

	if err := c.Login(cfg.Username, cfg.Password); err != nil {
		log.Warn().Err(err).Str("context_id", ctxID).Msg("inbox poll: login")
		return
	}

	if _, err := c.Select(imap.InboxName, true); err != nil {
		log.Warn().Err(err).Str("context_id", ctxID).Msg("inbox poll: select INBOX")
		return
	}

	rangeSpec := "1:*"
	if last > 0 {
		rangeSpec = fmt.Sprintf("%d:*", last+1)
	}
	seqset, err := imap.ParseSeqSet(rangeSpec)
	if err != nil {
		log.Warn().Err(err).Str("context_id", ctxID).Msg("inbox poll: build uid range")
		return
	}

	section, _ := imap.ParseBodySectionName(textSection)
	var msgs []*email.InboxMessage
	// go-imap delivers messages to the channel while UidFetch is in flight, so
	// consume it concurrently; an unbuffered channel read after the call
	// deadlocks.
	ch := make(chan *imap.Message)
	consumed := make(chan struct{})
	go func() {
		defer close(consumed)
		for msg := range ch {
			env := msg.Envelope
			if env == nil {
				continue
			}
			var body string
			if lit := msg.GetBody(section); lit != nil {
				if b, err := io.ReadAll(lit); err == nil {
					body = string(b)
				}
			}
			date := env.Date
			if date.IsZero() {
				date = time.Now()
			}
			msgs = append(msgs, &email.InboxMessage{
				ContextID:   ctxID,
				UID:         int64(msg.Uid),
				From:        addressesToString(env.From),
				To:          addressesToString(env.To),
				Subject:     env.Subject,
				BodyText:    body,
				MessageDate: date,
			})
		}
	}()
	if err := c.UidFetch(seqset, []imap.FetchItem{imap.FetchUid, imap.FetchEnvelope, textSection}, ch); err != nil {
		log.Warn().Err(err).Str("context_id", ctxID).Msg("inbox poll: fetch")
		<-consumed
		return
	}
	<-consumed

	// First run: keep only the most recent messages.
	if last == 0 && len(msgs) > firstRunCap {
		msgs = msgs[len(msgs)-firstRunCap:]
	}

	for _, m := range msgs {
		if err := p.inboxRepo.Upsert(m); err != nil {
			log.Warn().Err(err).Str("context_id", ctxID).Msg("inbox poll: upsert")
		}
	}

	if err := p.inboxRepo.TrimTo(ctxID, maxMessages); err != nil {
		log.Warn().Err(err).Str("context_id", ctxID).Msg("inbox poll: trim")
	}
}

// addressesToString renders IMAP addresses as "Name <addr>" joined by ", ".
func addressesToString(addrs []*imap.Address) string {
	parts := make([]string, 0, len(addrs))
	for _, a := range addrs {
		if a == nil {
			continue
		}
		if a.PersonalName != "" {
			parts = append(parts, fmt.Sprintf("%s <%s>", a.PersonalName, a.Address()))
		} else {
			parts = append(parts, a.Address())
		}
	}
	return strings.Join(parts, ", ")
}
