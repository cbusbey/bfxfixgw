// Package convert has utils to build FIX4.(2|4) messages to and from bitfinex
// API responses.
package convert

import (
	"strconv"

	"github.com/bitfinexcom/bfxfixgw/service/symbol"

	"github.com/bitfinexcom/bitfinex-api-go/v2"
	uuid "github.com/satori/go.uuid"

	"github.com/quickfixgo/enum"
	"github.com/quickfixgo/field"
	"github.com/quickfixgo/tag"
	"github.com/shopspring/decimal"

	fix42er "github.com/quickfixgo/fix42/executionreport"
	fix42mdir "github.com/quickfixgo/fix42/marketdataincrementalrefresh"
	fix42mdsfr "github.com/quickfixgo/fix42/marketdatasnapshotfullrefresh"
	ocj "github.com/quickfixgo/fix42/ordercancelreject"
	//fix42nos "github.com/quickfixgo/quickfix/fix42/newordersingle"
)

func FIX42MarketDataFullRefreshFromTradeSnapshot(mdReqID string, snapshot *bitfinex.TradeSnapshot, symbology symbol.Symbology, counterparty string) *fix42mdsfr.MarketDataSnapshotFullRefresh {
	if len(snapshot.Snapshot) <= 0 {
		return nil
	}
	first := snapshot.Snapshot[0]
	sym, err := symbology.FromBitfinex(first.Pair, counterparty)
	if err != nil {
		sym = first.Pair
	}
	message := fix42mdsfr.New(field.NewSymbol(sym))
	message.SetMDReqID(mdReqID)
	message.SetSymbol(sym)
	message.SetSecurityID(sym)
	message.SetIDSource(enum.IDSource_EXCHANGE_SYMBOL)
	// MDStreamID?
	group := fix42mdsfr.NewNoMDEntriesRepeatingGroup()
	for _, update := range snapshot.Snapshot {
		entry := group.Add()
		entry.SetMDEntryType(enum.MDEntryType_TRADE)
		entry.SetMDEntryPx(decimal.NewFromFloat(update.Price), 4)
		amt := update.Amount
		if amt < 0 {
			amt = -amt
		}
		entry.SetMDEntrySize(decimal.NewFromFloat(amt), 4)
	}
	message.SetNoMDEntries(group)
	return &message
}

func FIX42MarketDataFullRefreshFromBookSnapshot(mdReqID string, snapshot *bitfinex.BookUpdateSnapshot, symbology symbol.Symbology, counterparty string) *fix42mdsfr.MarketDataSnapshotFullRefresh {
	if len(snapshot.Snapshot) <= 0 {
		return nil
	}
	first := snapshot.Snapshot[0]
	sym, err := symbology.FromBitfinex(first.Symbol, counterparty)
	if err != nil {
		sym = first.Symbol
	}
	message := fix42mdsfr.New(field.NewSymbol(sym))
	message.SetMDReqID(mdReqID)
	message.SetSymbol(sym)
	message.SetSecurityID(sym)
	message.SetIDSource(enum.IDSource_EXCHANGE_SYMBOL)
	// MDStreamID?
	group := fix42mdsfr.NewNoMDEntriesRepeatingGroup()
	for _, update := range snapshot.Snapshot {
		entry := group.Add()
		var t enum.MDEntryType
		switch update.Side {
		case bitfinex.Bid:
			t = enum.MDEntryType_BID
		case bitfinex.Ask:
			t = enum.MDEntryType_OFFER
		}
		entry.SetMDEntryType(t)
		entry.SetMDEntryPx(decimal.NewFromFloat(update.Price), 4)
		amt := update.Amount
		if amt < 0 {
			amt = -amt
		}
		entry.SetMDEntrySize(decimal.NewFromFloat(amt), 4)
	}
	message.SetNoMDEntries(group)
	return &message
}

func FIX42MarketDataIncrementalRefreshFromTrade(mdReqID string, trade *bitfinex.Trade, symbology symbol.Symbology, counterparty string) *fix42mdir.MarketDataIncrementalRefresh {
	symbol, err := symbology.FromBitfinex(trade.Pair, counterparty)
	if err != nil {
		symbol = trade.Pair
	}

	message := fix42mdir.New()
	message.SetMDReqID(mdReqID)
	// MDStreamID?
	group := fix42mdir.NewNoMDEntriesRepeatingGroup()
	entry := group.Add()
	entry.SetMDEntryType(enum.MDEntryType_TRADE)
	entry.SetMDUpdateAction(enum.MDUpdateAction_NEW)
	entry.SetMDEntryPx(decimal.NewFromFloat(trade.Price), 4)
	entry.SetSecurityID(symbol)
	entry.SetIDSource(enum.IDSource_EXCHANGE_SYMBOL)
	amt := trade.Amount
	if amt < 0 {
		amt = -amt
	}
	entry.SetMDEntrySize(decimal.NewFromFloat(amt), 4)
	entry.SetSymbol(symbol)
	message.SetNoMDEntries(group)
	return &message
}

func FIX42MarketDataIncrementalRefreshFromBookUpdate(mdReqID string, update *bitfinex.BookUpdate, symbology symbol.Symbology, counterparty string) *fix42mdir.MarketDataIncrementalRefresh {
	symbol, err := symbology.FromBitfinex(update.Symbol, counterparty)
	if err != nil {
		symbol = update.Symbol
	}

	message := fix42mdir.New()
	message.SetMDReqID(mdReqID)
	// MDStreamID?
	group := fix42mdir.NewNoMDEntriesRepeatingGroup()
	entry := group.Add()
	var t enum.MDEntryType
	switch update.Side {
	case bitfinex.Bid:
		t = enum.MDEntryType_BID
	case bitfinex.Ask:
		t = enum.MDEntryType_OFFER
	}
	action := BookActionToFIX(update.Action)
	entry.SetMDEntryType(t)
	entry.SetMDUpdateAction(action)
	entry.SetMDEntryPx(decimal.NewFromFloat(update.Price), 4)
	entry.SetSecurityID(symbol)
	entry.SetIDSource(enum.IDSource_EXCHANGE_SYMBOL)
	amt := update.Amount
	if amt < 0 {
		amt = -amt
	}
	if action != enum.MDUpdateAction_DELETE {
		entry.SetMDEntrySize(decimal.NewFromFloat(amt), 4)
	}
	entry.SetSymbol(symbol)
	message.SetNoMDEntries(group)
	return &message
}

func FIX42ExecutionReport(symbol, clOrdID, orderID, account string, execType enum.ExecType, side enum.Side, origQty, thisQty, cumQty, px, stop, trail, avgPx float64, ordStatus enum.OrdStatus, ordType enum.OrdType, tif enum.TimeInForce, text string, symbology symbol.Symbology, counterparty string, flags int) fix42er.ExecutionReport {
	uid, err := uuid.NewV4()
	execID := ""
	if err == nil {
		execID = uid.String()
	}
	// total order qty
	amt := decimal.NewFromFloat(origQty)

	// total executed so far
	cumAmt := decimal.NewFromFloat(cumQty)

	// remaining to be executed
	remaining := amt.Sub(cumAmt)
	switch ordStatus {
	case enum.OrdStatus_CANCELED:
		fallthrough
	case enum.OrdStatus_DONE_FOR_DAY:
		fallthrough
	case enum.OrdStatus_EXPIRED:
		fallthrough
	case enum.OrdStatus_REPLACED:
		fallthrough
	case enum.OrdStatus_STOPPED:
		fallthrough
	case enum.OrdStatus_SUSPENDED:
		remaining = decimal.Zero
	}

	// this execution
	lastShares := decimal.NewFromFloat(thisQty)

	sym, err := symbology.FromBitfinex(symbol, counterparty)
	if err != nil {
		sym = symbol
	}

	e := fix42er.New(
		field.NewOrderID(orderID),
		field.NewExecID(execID),
		field.NewExecTransType(enum.ExecTransType_STATUS),
		field.NewExecType(execType),
		field.NewOrdStatus(ordStatus),
		field.NewSymbol(sym),
		field.NewSide(side),
		field.NewLeavesQty(remaining, 4), // qty
		field.NewCumQty(cumAmt, 4),
		AvgPxToFIX(avgPx),
	)
	e.SetAccount(account)
	if lastShares.Cmp(decimal.Zero) != 0 {
		e.SetLastShares(lastShares, 4)
	}
	e.SetOrderQty(amt, 4)
	if text != "" {
		e.SetText(text)
	}
	e.SetOrdType(ordType)
	e.SetClOrdID(clOrdID)

	switch ordType {
	case enum.OrdType_LIMIT:
		if px != 0 {
			e.SetPrice(decimal.NewFromFloat(px), 4)
		}
	case enum.OrdType_STOP_LIMIT:
		if px != 0 {
			e.SetPrice(decimal.NewFromFloat(px), 4)
		}
		if stop != 0 {
			e.SetStopPx(decimal.NewFromFloat(stop), 4)
		}
	case enum.OrdType_STOP:
		if stop != 0 {
			e.SetStopPx(decimal.NewFromFloat(stop), 4)
		}
	}

	execInst := ""
	if trail != 0 {
		execInst = string(enum.ExecInst_PRIMARY_PEG)
		e.SetPegDifference(decimal.NewFromFloat(trail), 4)
	}
	if flags&FlagHidden != 0 {
		e.SetString(tag.DisplayMethod, string(enum.DisplayMethod_UNDISCLOSED))
	}
	if flags&FlagPostOnly != 0 {
		execInst = execInst + string(enum.ExecInst_PARTICIPANT_DONT_INITIATE)
	}
	if execInst != "" {
		e.SetExecInst(enum.ExecInst(execInst))
	}
	e.SetTimeInForce(tif)

	return e
}

func FIX42ExecutionReportFromOrder(o *bitfinex.Order, account string, execType enum.ExecType, cumQty float64, ordStatus enum.OrdStatus, text string, symbology symbol.Symbology, counterparty string, flags int, stop, peg float64) fix42er.ExecutionReport {
	orderID := strconv.FormatInt(o.ID, 10)
	// total order qty
	fAmt := o.Amount
	if fAmt < 0 {
		fAmt = -fAmt
	}
	ordtype := OrdTypeToFIX(bitfinex.OrderType(o.Type))
	tif := TimeInForceToFIX(bitfinex.OrderType(o.Type)) // support FOK
	e := FIX42ExecutionReport(o.Symbol, strconv.FormatInt(o.CID, 10), orderID, account, execType, SideToFIX(o.Amount), fAmt, 0.0, cumQty, o.Price, stop, peg, o.PriceAvg, ordStatus, ordtype, tif, text, symbology, counterparty, flags)
	if text != "" {
		e.SetText(text)
	}
	e.SetLastShares(decimal.Zero, 4) // qty
	return e
}

func FIX42ExecutionReportFromTradeExecutionUpdate(t *bitfinex.TradeExecutionUpdate, account, clOrdID string, origQty, totalFillQty, origPx, stopPx, trailPx, avgFillPx float64, symbology symbol.Symbology, counterparty string, flags int) fix42er.ExecutionReport {
	orderID := strconv.FormatInt(t.OrderID, 10)
	var execType enum.ExecType
	var ordStatus enum.OrdStatus
	if totalFillQty >= origQty {
		execType = enum.ExecType_FILL
		ordStatus = enum.OrdStatus_FILLED
	} else {
		execType = enum.ExecType_PARTIAL_FILL
		ordStatus = enum.OrdStatus_PARTIALLY_FILLED
	}
	execAmt := t.ExecAmount
	if execAmt < 0 {
		execAmt = -execAmt
	}
	tif := TimeInForceToFIX(bitfinex.OrderType(t.OrderType)) // support FOK
	er := FIX42ExecutionReport(t.Pair, clOrdID, orderID, account, execType, SideToFIX(t.ExecAmount), origQty, execAmt, totalFillQty, origPx, stopPx, trailPx, avgFillPx, ordStatus, OrdTypeToFIX(bitfinex.OrderType(t.OrderType)), tif, "", symbology, counterparty, flags)
	f := t.Fee
	if f < 0 {
		f = -f
	}

	// trade-specific
	fee := decimal.NewFromFloat(f)
	er.SetCommission(fee, 4)
	er.SetCommType(enum.CommType_ABSOLUTE)
	er.SetLastPx(decimal.NewFromFloat(t.ExecPrice), 4)
	return er
}

func rejectReasonFromText(text string) enum.CxlRejReason {
	switch text {
	case "Order not found.":
		return enum.CxlRejReason_UNKNOWN_ORDER
	}
	return enum.CxlRejReason_OTHER
}

func FIX42OrderCancelReject(account, orderID, origClOrdID, cxlClOrdID, text string) ocj.OrderCancelReject {
	rejReason := rejectReasonFromText(text)
	if rejReason == enum.CxlRejReason_UNKNOWN_ORDER {
		orderID = "NONE" // FIX spec tag 37 in 35=9: If CxlRejReason="Unknown order", specify "NONE".
	}
	r := ocj.New(
		field.NewOrderID(orderID),
		field.NewClOrdID(cxlClOrdID),
		field.NewOrigClOrdID(origClOrdID),
		field.NewOrdStatus(enum.OrdStatus_REJECTED),
		field.NewCxlRejResponseTo(enum.CxlRejResponseTo_ORDER_CANCEL_REQUEST),
	)
	r.SetCxlRejReason(rejReason)
	r.SetAccount(account)
	r.SetText(text)
	return r
}

func FIX42NoMDEntriesRepeatingGroupFromTradeTicker(data []float64) fix42mdsfr.NoMDEntriesRepeatingGroup {
	mdEntriesGroup := fix42mdsfr.NewNoMDEntriesRepeatingGroup()

	mde := mdEntriesGroup.Add()
	mde.SetMDEntryType(enum.MDEntryType_BID)
	mde.SetMDEntryPx(decimal.NewFromFloat(data[0]), 2)
	mde.SetMDEntrySize(decimal.NewFromFloat(data[1]), 3)

	mde = mdEntriesGroup.Add()
	mde.SetMDEntryType(enum.MDEntryType_OFFER)
	mde.SetMDEntryPx(decimal.NewFromFloat(data[2]), 2)
	mde.SetMDEntrySize(decimal.NewFromFloat(data[3]), 3)

	mde = mdEntriesGroup.Add()
	mde.SetMDEntryType(enum.MDEntryType_TRADE)
	mde.SetMDEntryPx(decimal.NewFromFloat(data[6]), 2)

	mde = mdEntriesGroup.Add()
	mde.SetMDEntryType(enum.MDEntryType_TRADE_VOLUME)
	mde.SetMDEntrySize(decimal.NewFromFloat(data[7]), 8)

	mde = mdEntriesGroup.Add()
	mde.SetMDEntryType(enum.MDEntryType_TRADING_SESSION_HIGH_PRICE)
	mde.SetMDEntrySize(decimal.NewFromFloat(data[8]), 2)

	mde = mdEntriesGroup.Add()
	mde.SetMDEntryType(enum.MDEntryType_TRADING_SESSION_LOW_PRICE)
	mde.SetMDEntrySize(decimal.NewFromFloat(data[9]), 2)

	return mdEntriesGroup
}
