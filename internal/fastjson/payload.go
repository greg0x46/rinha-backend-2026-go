package fastjson

import "bytes"

// Payload is the parsed shape of a fraud-score request, holding only the
// values the vectorizer actually consumes. Byte-slice fields (MerchantID,
// MCC, KnownMerchants[i]) point into the original body, so the caller
// must not mutate the body until the vector is built.
type Payload struct {
	Amount            float64
	Installments      int
	RequestedAt       Timestamp
	HasRequestedAt    bool

	AvgAmount  float64
	TxCount24h int

	KnownMerchants    [][]byte // reused; reset Len to 0 each call

	MerchantID        []byte
	MCC               []byte
	MerchantAvgAmount float64

	IsOnline    bool
	CardPresent bool
	KmFromHome  float64

	HasLastTransaction bool
	LastTimestamp      Timestamp
	LastKmFromCurrent  float64
}

// Reset prepares an existing Payload for reuse (e.g. from a sync.Pool)
// while preserving any underlying capacity in KnownMerchants.
func (p *Payload) Reset() {
	known := p.KnownMerchants[:0]
	*p = Payload{}
	p.KnownMerchants = known
}

// Parse decodes body into out. Returns an error if the payload uses any
// JSON feature outside the fraud-score contract (string escapes,
// scientific-notation numbers, malformed timestamps). Callers typically
// fall back to encoding/json on error.
func Parse(body []byte, out *Payload) error {
	p := parser{data: body}
	if err := p.parseTopLevel(out); err != nil {
		return err
	}
	p.skipWS()
	if !p.eof() {
		return ErrSyntax
	}
	return nil
}

func (p *parser) parseTopLevel(out *Payload) error {
	if err := p.expect('{'); err != nil {
		return err
	}
	p.skipWS()
	if p.peek() == '}' {
		p.pos++
		return nil
	}
	for {
		key, err := p.parseString()
		if err != nil {
			return err
		}
		if err := p.expect(':'); err != nil {
			return err
		}
		switch {
		case bytes.Equal(key, keyTransaction):
			if err := p.parseTransaction(out); err != nil {
				return err
			}
		case bytes.Equal(key, keyCustomer):
			if err := p.parseCustomer(out); err != nil {
				return err
			}
		case bytes.Equal(key, keyMerchant):
			if err := p.parseMerchant(out); err != nil {
				return err
			}
		case bytes.Equal(key, keyTerminal):
			if err := p.parseTerminal(out); err != nil {
				return err
			}
		case bytes.Equal(key, keyLastTransaction):
			if err := p.parseLastTransaction(out); err != nil {
				return err
			}
		default:
			if err := p.skipValue(); err != nil {
				return err
			}
		}
		p.skipWS()
		if p.peek() == ',' {
			p.pos++
			continue
		}
		return p.expect('}')
	}
}

func (p *parser) parseTransaction(out *Payload) error {
	if err := p.expect('{'); err != nil {
		return err
	}
	p.skipWS()
	if p.peek() == '}' {
		p.pos++
		return nil
	}
	for {
		key, err := p.parseString()
		if err != nil {
			return err
		}
		if err := p.expect(':'); err != nil {
			return err
		}
		switch {
		case bytes.Equal(key, keyAmount):
			v, err := p.parseNumber()
			if err != nil {
				return err
			}
			out.Amount = v
		case bytes.Equal(key, keyInstallments):
			v, err := p.parseInt()
			if err != nil {
				return err
			}
			out.Installments = v
		case bytes.Equal(key, keyRequestedAt):
			s, err := p.parseString()
			if err != nil {
				return err
			}
			ts, ok := parseTimestamp(s)
			if !ok {
				return ErrSyntax
			}
			out.RequestedAt = ts
			out.HasRequestedAt = true
		default:
			if err := p.skipValue(); err != nil {
				return err
			}
		}
		p.skipWS()
		if p.peek() == ',' {
			p.pos++
			continue
		}
		return p.expect('}')
	}
}

func (p *parser) parseCustomer(out *Payload) error {
	if err := p.expect('{'); err != nil {
		return err
	}
	p.skipWS()
	if p.peek() == '}' {
		p.pos++
		return nil
	}
	for {
		key, err := p.parseString()
		if err != nil {
			return err
		}
		if err := p.expect(':'); err != nil {
			return err
		}
		switch {
		case bytes.Equal(key, keyAvgAmount):
			v, err := p.parseNumber()
			if err != nil {
				return err
			}
			out.AvgAmount = v
		case bytes.Equal(key, keyTxCount24h):
			v, err := p.parseInt()
			if err != nil {
				return err
			}
			out.TxCount24h = v
		case bytes.Equal(key, keyKnownMerchants):
			if err := p.parseKnownMerchants(out); err != nil {
				return err
			}
		default:
			if err := p.skipValue(); err != nil {
				return err
			}
		}
		p.skipWS()
		if p.peek() == ',' {
			p.pos++
			continue
		}
		return p.expect('}')
	}
}

func (p *parser) parseKnownMerchants(out *Payload) error {
	if err := p.expect('['); err != nil {
		return err
	}
	p.skipWS()
	out.KnownMerchants = out.KnownMerchants[:0]
	if p.peek() == ']' {
		p.pos++
		return nil
	}
	for {
		s, err := p.parseString()
		if err != nil {
			return err
		}
		out.KnownMerchants = append(out.KnownMerchants, s)
		p.skipWS()
		if p.peek() == ',' {
			p.pos++
			continue
		}
		return p.expect(']')
	}
}

func (p *parser) parseMerchant(out *Payload) error {
	if err := p.expect('{'); err != nil {
		return err
	}
	p.skipWS()
	if p.peek() == '}' {
		p.pos++
		return nil
	}
	for {
		key, err := p.parseString()
		if err != nil {
			return err
		}
		if err := p.expect(':'); err != nil {
			return err
		}
		switch {
		case bytes.Equal(key, keyID):
			s, err := p.parseString()
			if err != nil {
				return err
			}
			out.MerchantID = s
		case bytes.Equal(key, keyMCC):
			s, err := p.parseString()
			if err != nil {
				return err
			}
			out.MCC = s
		case bytes.Equal(key, keyAvgAmount):
			v, err := p.parseNumber()
			if err != nil {
				return err
			}
			out.MerchantAvgAmount = v
		default:
			if err := p.skipValue(); err != nil {
				return err
			}
		}
		p.skipWS()
		if p.peek() == ',' {
			p.pos++
			continue
		}
		return p.expect('}')
	}
}

func (p *parser) parseTerminal(out *Payload) error {
	if err := p.expect('{'); err != nil {
		return err
	}
	p.skipWS()
	if p.peek() == '}' {
		p.pos++
		return nil
	}
	for {
		key, err := p.parseString()
		if err != nil {
			return err
		}
		if err := p.expect(':'); err != nil {
			return err
		}
		switch {
		case bytes.Equal(key, keyIsOnline):
			v, err := p.parseBool()
			if err != nil {
				return err
			}
			out.IsOnline = v
		case bytes.Equal(key, keyCardPresent):
			v, err := p.parseBool()
			if err != nil {
				return err
			}
			out.CardPresent = v
		case bytes.Equal(key, keyKmFromHome):
			v, err := p.parseNumber()
			if err != nil {
				return err
			}
			out.KmFromHome = v
		default:
			if err := p.skipValue(); err != nil {
				return err
			}
		}
		p.skipWS()
		if p.peek() == ',' {
			p.pos++
			continue
		}
		return p.expect('}')
	}
}

func (p *parser) parseLastTransaction(out *Payload) error {
	p.skipWS()
	if p.peek() == 'n' {
		if !p.consumeNull() {
			return ErrSyntax
		}
		out.HasLastTransaction = false
		return nil
	}
	if err := p.expect('{'); err != nil {
		return err
	}
	out.HasLastTransaction = true
	p.skipWS()
	if p.peek() == '}' {
		p.pos++
		return nil
	}
	for {
		key, err := p.parseString()
		if err != nil {
			return err
		}
		if err := p.expect(':'); err != nil {
			return err
		}
		switch {
		case bytes.Equal(key, keyTimestamp):
			s, err := p.parseString()
			if err != nil {
				return err
			}
			ts, ok := parseTimestamp(s)
			if !ok {
				return ErrSyntax
			}
			out.LastTimestamp = ts
		case bytes.Equal(key, keyKmFromCurrent):
			v, err := p.parseNumber()
			if err != nil {
				return err
			}
			out.LastKmFromCurrent = v
		default:
			if err := p.skipValue(); err != nil {
				return err
			}
		}
		p.skipWS()
		if p.peek() == ',' {
			p.pos++
			continue
		}
		return p.expect('}')
	}
}

var (
	keyTransaction     = []byte("transaction")
	keyCustomer        = []byte("customer")
	keyMerchant        = []byte("merchant")
	keyTerminal        = []byte("terminal")
	keyLastTransaction = []byte("last_transaction")

	keyAmount       = []byte("amount")
	keyInstallments = []byte("installments")
	keyRequestedAt  = []byte("requested_at")

	keyAvgAmount      = []byte("avg_amount")
	keyTxCount24h     = []byte("tx_count_24h")
	keyKnownMerchants = []byte("known_merchants")

	keyID  = []byte("id")
	keyMCC = []byte("mcc")

	keyIsOnline    = []byte("is_online")
	keyCardPresent = []byte("card_present")
	keyKmFromHome  = []byte("km_from_home")

	keyTimestamp     = []byte("timestamp")
	keyKmFromCurrent = []byte("km_from_current")
)
