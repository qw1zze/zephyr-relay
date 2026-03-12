package relay

func (r *Relay) Route(msg Message) {
	r.log.Info("route stub called", "type", msg.Type)
}
