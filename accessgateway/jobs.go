package accessgateway

func (g *AccessGateway) runJobs() { g.scheduler.StartAsync() }
