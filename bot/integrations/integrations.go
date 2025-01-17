package integrations

import (
	"github.com/jadevelopmentgrp/Tickets-Utilities/integrations/bloxlink"
	"github.com/jadevelopmentgrp/Tickets-Utilities/webproxy"
	"github.com/jadevelopmentgrp/Tickets-Worker/bot/redis"
	"github.com/jadevelopmentgrp/Tickets-Worker/config"
)

var (
	WebProxy    *webproxy.WebProxy
	SecureProxy *SecureProxyClient
	Bloxlink    *bloxlink.BloxlinkIntegration
)

func InitIntegrations() {
	WebProxy = webproxy.NewWebProxy(config.Conf.WebProxy.Url, config.Conf.WebProxy.AuthHeaderName, config.Conf.WebProxy.AuthHeaderValue)
	Bloxlink = bloxlink.NewBloxlinkIntegration(redis.Client, WebProxy, config.Conf.Integrations.BloxlinkApiKey)
	SecureProxy = NewSecureProxy(config.Conf.Integrations.SecureProxyUrl)
}
