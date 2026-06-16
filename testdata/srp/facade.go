package srp

// LargeFacade models the real-world facade/aggregate pattern (cobra.Command,
// gin.Engine, fasthttp.Request): a large type whose many methods cluster into a
// couple of cohesive areas. Here the methods split into two field groups —
// request-side (reqURL/reqBody/reqHdr) and response-side (respCode/respBody/
// respHdr) — so LCOM4 = 2, but each group holds several methods (high average
// methods-per-group). LCOM4 counts the two groups but not their size, so a flat
// cohesion penalty would floor this type on the same evidence that flags a small
// bag of unrelated methods (GodStruct). The graduated penalty attenuates the
// cohesion term for such large aggregates, while the method-count/complexity
// penalties still keep the score moderate (below the SRP threshold) rather than
// perfect — an honest "large, but not incohesive" verdict.
// solid:want SRP=na reason="facade vs true god-object is structurally ambiguous (same LCOM4=2, large avg group); classification deferred to Phase 4 (design §7)"
type LargeFacade struct {
	reqURL   string
	reqBody  []byte
	reqHdr   map[string]string
	respCode int
	respBody []byte
	respHdr  map[string]string
}

// --- request-side group (all touch req* fields) ---

func (f *LargeFacade) SetURL(u string)           { f.reqURL = u }
func (f *LargeFacade) URL() string               { return f.reqURL }
func (f *LargeFacade) SetReqBody(b []byte)       { f.reqBody = b }
func (f *LargeFacade) ReqBody() []byte           { return f.reqBody }
func (f *LargeFacade) SetReqHeader(k, v string)  { f.reqHdr[k] = v }
func (f *LargeFacade) ReqHeader(k string) string { return f.reqHdr[k] }
func (f *LargeFacade) ClearReq()                 { f.reqURL = ""; f.reqBody = nil; f.reqHdr = nil }
func (f *LargeFacade) HasReqBody() bool          { return len(f.reqBody) > 0 }

// --- response-side group (all touch resp* fields) ---

func (f *LargeFacade) SetStatus(c int)            { f.respCode = c }
func (f *LargeFacade) Status() int                { return f.respCode }
func (f *LargeFacade) SetRespBody(b []byte)       { f.respBody = b }
func (f *LargeFacade) RespBody() []byte           { return f.respBody }
func (f *LargeFacade) SetRespHeader(k, v string)  { f.respHdr[k] = v }
func (f *LargeFacade) RespHeader(k string) string { return f.respHdr[k] }
func (f *LargeFacade) ClearResp()                 { f.respCode = 0; f.respBody = nil; f.respHdr = nil }
func (f *LargeFacade) HasRespBody() bool          { return len(f.respBody) > 0 }

// SmallSplit is a small type whose methods split into two groups (LCOM4=2) but
// whose average group size (~3.5 methods) is only marginally above the
// attenuation threshold, so its cohesion penalty is barely reduced. It is *not*
// a large structured aggregate, so — unlike LargeFacade — its SRP confidence
// must stay high: the confidence drop is reserved for substantially attenuated
// aggregates, not for any type that is fractionally attenuated.
// solid:want SRP=ok reason="two small cohesive groups (a-methods, b-methods); minor split, not the many-small-islands god-object smell"
type SmallSplit struct {
	a int
	b int
}

func (s *SmallSplit) IncA()   { s.a++ }
func (s *SmallSplit) DecA()   { s.a-- }
func (s *SmallSplit) A() int  { return s.a }
func (s *SmallSplit) ResetA() { s.a = 0 }
func (s *SmallSplit) IncB()   { s.b++ }
func (s *SmallSplit) DecB()   { s.b-- }
func (s *SmallSplit) B() int  { return s.b }
