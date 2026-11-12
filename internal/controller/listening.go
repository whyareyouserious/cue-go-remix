package controller

func (a *App) StartListening() error  { return a.runtime.StartListening() }
func (a *App) StopListening() error   { return a.runtime.StopListening() }
func (a *App) ClearTranscript() error { return a.runtime.ClearTranscript() }
