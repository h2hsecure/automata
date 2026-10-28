package cicd_test

import (
	"github.com/h2hsecure/automata/cicd/support"
	mailsmtp "github.com/h2hsecure/automata/internal/infrastructure/email/smtp"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("SMTP settings merge", func() {
	base := mailsmtp.Config{Host: "yaml.host", Port: 587, From: "yaml@x", StartTLS: true}

	Describe("MergeConfig", func() {
		It("applies non-empty overrides", func() {
			got := mailsmtp.MergeConfig(base, map[string]string{
				"smtp.host": "db.host", "smtp.port": "2525", "smtp.from": "db@x",
			})
			Expect(got.Host).To(Equal("db.host"))
			Expect(got.Port).To(Equal(2525))
			Expect(got.From).To(Equal("db@x"))
			Expect(got.StartTLS).To(BeTrue()) // untouched
		})

		It("ignores a non-numeric port", func() {
			got := mailsmtp.MergeConfig(base, map[string]string{"smtp.port": "abc"})
			Expect(got.Port).To(Equal(587))
		})

		It("honors starttls=false and keeps base when overrides are empty", func() {
			got := mailsmtp.MergeConfig(base, map[string]string{"smtp.starttls": "false"})
			Expect(got.StartTLS).To(BeFalse())
			Expect(mailsmtp.MergeConfig(base, map[string]string{})).To(Equal(base))
		})
	})

	Describe("provider mailer", func() {
		It("resolves config from the provider at send time", func() {
			mock := support.NewMockSMTP()
			mock.Start()
			defer mock.Stop()
			var got mailsmtp.Config
			mailer := mailsmtp.NewMailerWithProvider(
				func(ctxID string) mailsmtp.Config { return mailsmtp.Config{Host: "db", From: "db@x"} },
				base,
				func(cfg mailsmtp.Config) mailsmtp.Dialer {
					got = cfg
					return &dialerAdapter{d: support.NewSMTPDialer(mock)}
				},
			)
			Expect(mailer.Send("ctx-1", []string{"a@b.c"}, "s", "b", "")).To(Succeed())
			Expect(got.From).To(Equal("db@x"))
			Expect(got.Host).To(Equal("db"))
		})

		It("treats an unconfigured provider result as not configured", func() {
			mailer := mailsmtp.NewMailerWithProvider(
				func(ctxID string) mailsmtp.Config { return mailsmtp.Config{} },
				base,
				func(cfg mailsmtp.Config) mailsmtp.Dialer { return nil },
			)
			err := mailer.Send("ctx-1", []string{"a@b.c"}, "s", "b", "")
			Expect(err).To(MatchError(ContainSubstring("not configured")))
		})
	})
})
