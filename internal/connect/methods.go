package connect

func (c *Connector) registerAllMethods() {
	c.registerMetaMethods()
	c.registerChatMethods()
	c.registerSTTMethods()
	c.registerSTTStreamMethods()
	c.registerSessionMethods()
	c.registerSettingsMethods()
	c.registerMentionMethods()
}
