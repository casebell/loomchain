// +build evm

package query

import (
	"bytes"
	"testing"

	"github.com/loomnetwork/loomchain/events"
	"github.com/loomnetwork/loomchain/rpc/eth"
	"github.com/loomnetwork/loomchain/store"

	"github.com/loomnetwork/loomchain/receipts/common"

	"github.com/gogo/protobuf/proto"
	"github.com/loomnetwork/go-loom"
	"github.com/loomnetwork/go-loom/plugin/types"
	types1 "github.com/loomnetwork/go-loom/types"
	"github.com/loomnetwork/loomchain"
	"github.com/loomnetwork/loomchain/eth/bloom"
	"github.com/loomnetwork/loomchain/eth/utils"
	"github.com/loomnetwork/loomchain/receipts/handler"
	"github.com/stretchr/testify/require"
)

const (
	allFilter = "{\"fromBlock\":\"earliest\",\"toBlock\":\"latest\",\"address\":\"\",\"topics\":[]}"
)

var (
	addr1 = loom.MustParseAddress("chain:0xb16a379ec18d4093666f8f38b11a3071c920207d")
	addr2 = loom.MustParseAddress("chain:0x5cecd1f7261e1f4c684e297be3edf03b825e01c4")
)

func TestQueryChain(t *testing.T) {
	testQueryChain(t, handler.ReceiptHandlerLevelDb)
}

func testQueryChain(t *testing.T, v handler.ReceiptHandlerVersion) {
	evmAuxStore, err := common.NewMockEvmAuxStore()
	require.NoError(t, err)
	eventDispatcher := events.NewLogEventDispatcher()
	eventHandler := loomchain.NewDefaultEventHandler(eventDispatcher)
	receiptHandler := handler.NewReceiptHandler(eventHandler, handler.DefaultMaxReceipts, evmAuxStore)
	var writer loomchain.WriteReceiptHandler = receiptHandler

	require.NoError(t, err)
	state := common.MockState(0)

	state4 := common.MockStateAt(state, 4)
	mockEvent1 := []*types.EventData{
		{
			Topics:      []string{"topic1", "topic2", "topic3"},
			EncodedBody: []byte("somedata"),
			Address:     addr1.MarshalPB(),
		},
	}
	_, err = writer.CacheReceipt(state4, addr1, addr2, mockEvent1, nil)
	require.NoError(t, err)
	receiptHandler.CommitCurrentReceipt()

	protoBlock, err := GetPendingBlock(4, true, receiptHandler)
	require.NoError(t, err)
	blockInfo := types.EthBlockInfo{}
	require.NoError(t, proto.Unmarshal(protoBlock, &blockInfo))
	require.EqualValues(t, int64(4), blockInfo.Number)
	require.EqualValues(t, 1, len(blockInfo.Transactions))

	require.NoError(t, receiptHandler.CommitBlock(4))

	mockEvent2 := []*types.EventData{
		{
			Topics:      []string{"topic1"},
			EncodedBody: []byte("somedata"),
			Address:     addr1.MarshalPB(),
		},
	}
	state20 := common.MockStateAt(state, 20)
	_, err = writer.CacheReceipt(state20, addr1, addr2, mockEvent2, nil)
	require.NoError(t, err)
	receiptHandler.CommitCurrentReceipt()
	require.NoError(t, receiptHandler.CommitBlock(20))

	blockStore := store.NewMockBlockStore()

	state30 := common.MockStateAt(state, uint64(30))
	result, err := DeprecatedQueryChain(allFilter, blockStore, state30, receiptHandler, evmAuxStore)
	require.NoError(t, err, "error query chain, filter is %s", allFilter)
	var logs types.EthFilterLogList
	require.NoError(t, proto.Unmarshal(result, &logs), "unmarshalling EthFilterLogList")
	require.Equal(t, 2, len(logs.EthBlockLogs), "wrong number of logs returned")

	ethFilter, err := utils.UnmarshalEthFilter([]byte(allFilter))
	require.NoError(t, err)
	filterLogs, err := QueryChain(blockStore, state30, ethFilter, receiptHandler, evmAuxStore)
	require.NoError(t, err, "error query chain, filter is %s", ethFilter)
	require.Equal(t, 2, len(filterLogs), "wrong number of logs returned")

	require.NoError(t, receiptHandler.Close())
}

func TestMatchFilters(t *testing.T) {
	addr3 := &types1.Address{
		ChainId: "defult",
		Local:   []byte("test3333"),
	}
	addr4 := &types1.Address{
		ChainId: "defult",
		Local:   []byte("test4444"),
	}
	testEvents := []*loomchain.EventData{
		{
			Topics:  []string{"Topic1", "Topic2", "Topic3", "Topic4"},
			Address: addr3,
		},
		{
			Topics:  []string{"Topic5"},
			Address: addr3,
		},
	}
	testEventsG := []*types.EventData{
		{
			Topics:      []string{"Topic1", "Topic2", "Topic3", "Topic4"},
			Address:     addr3,
			EncodedBody: []byte("Some data"),
		},
		{
			Topics:  []string{"Topic5"},
			Address: addr3,
		},
	}
	ethFilter1 := eth.EthBlockFilter{
		Topics: [][]string{{"Topic1"}, nil, {"Topic3", "Topic4"}, {"Topic4"}},
	}
	ethFilter2 := eth.EthBlockFilter{
		Addresses: []loom.LocalAddress{addr4.Local},
	}
	ethFilter3 := eth.EthBlockFilter{
		Topics: [][]string{{"Topic1"}},
	}
	ethFilter4 := eth.EthBlockFilter{
		Addresses: []loom.LocalAddress{addr4.Local, addr3.Local},
		Topics:    [][]string{nil, nil, {"Topic2"}},
	}
	ethFilter5 := eth.EthBlockFilter{
		Topics: [][]string{{"Topic1"}, {"Topic6"}},
	}
	bloomFilter := bloom.GenBloomFilter(common.ConvertEventData(testEvents))

	require.True(t, MatchBloomFilter(ethFilter1, bloomFilter))
	require.False(t, MatchBloomFilter(ethFilter2, bloomFilter)) // address does not match
	require.True(t, MatchBloomFilter(ethFilter3, bloomFilter))  // one of the addresses mathch
	require.True(t, MatchBloomFilter(ethFilter4, bloomFilter))
	require.False(t, MatchBloomFilter(ethFilter5, bloomFilter))

	require.True(t, utils.MatchEthFilter(ethFilter1, *testEventsG[0]))
	require.False(t, utils.MatchEthFilter(ethFilter2, *testEventsG[0]))
	require.True(t, utils.MatchEthFilter(ethFilter3, *testEventsG[0]))
	require.False(t, utils.MatchEthFilter(ethFilter4, *testEventsG[0]))
	require.False(t, utils.MatchEthFilter(ethFilter5, *testEventsG[0]))

	require.False(t, utils.MatchEthFilter(ethFilter1, *testEventsG[1]))
	require.False(t, utils.MatchEthFilter(ethFilter2, *testEventsG[1]))
	require.False(t, utils.MatchEthFilter(ethFilter3, *testEventsG[1]))
	require.False(t, utils.MatchEthFilter(ethFilter4, *testEventsG[1]))
	require.False(t, utils.MatchEthFilter(ethFilter5, *testEventsG[1]))
}

func TestGetLogs(t *testing.T) {
	testGetLogs(t, handler.ReceiptHandlerLevelDb)
}

func testGetLogs(t *testing.T, v handler.ReceiptHandlerVersion) {
	evmAuxStore, err := common.NewMockEvmAuxStore()
	require.NoError(t, err)

	eventDispatcher := events.NewLogEventDispatcher()
	eventHandler := loomchain.NewDefaultEventHandler(eventDispatcher)
	receiptHandler := handler.NewReceiptHandler(eventHandler, handler.DefaultMaxReceipts, evmAuxStore)
	var writer loomchain.WriteReceiptHandler = receiptHandler

	require.NoError(t, err)
	ethFilter := eth.EthBlockFilter{
		Topics: [][]string{{"Topic1"}, nil, {"Topic3", "Topic4"}, {"Topic4"}},
	}
	testEvents := []*types.EventData{
		{
			Topics:      []string{"Topic1", "Topic2", "Topic3", "Topic4"},
			Address:     addr1.MarshalPB(),
			EncodedBody: []byte("Some data"),
		},
		{
			Topics:  []string{"Topic5"},
			Address: addr1.MarshalPB(),
		},
	}

	testEventsG := []*types.EventData{
		{
			Topics:      []string{"Topic1", "Topic2", "Topic3", "Topic4"},
			Address:     addr1.MarshalPB(),
			EncodedBody: []byte("Some data"),
		},
		{
			Topics:  []string{"Topic5"},
			Address: addr1.MarshalPB(),
		},
	}
	state := common.MockState(1)
	state32 := common.MockStateAt(state, 32)
	txHash, err := writer.CacheReceipt(state32, addr1, addr2, testEventsG, nil)
	require.NoError(t, err)
	receiptHandler.CommitCurrentReceipt()
	require.NoError(t, receiptHandler.CommitBlock(32))

	txReceipt, err := receiptHandler.GetReceipt(txHash)
	require.NoError(t, err)

	blockStore := store.NewMockBlockStore()

	logs, err := getTxHashLogs(blockStore, txReceipt, ethFilter, txHash)
	require.NoError(t, err, "getBlockLogs failed")
	require.Equal(t, len(logs), 1)
	require.Equal(t, logs[0].TransactionIndex, txReceipt.TransactionIndex)
	require.Equal(t, logs[0].TransactionHash, txReceipt.TxHash)
	require.True(t, 0 == bytes.Compare(logs[0].BlockHash, txReceipt.BlockHash))
	require.Equal(t, logs[0].BlockNumber, txReceipt.BlockNumber)
	require.True(t, 0 == bytes.Compare(logs[0].Address, txReceipt.CallerAddress.Local))
	require.True(t, 0 == bytes.Compare(logs[0].Data, testEvents[0].EncodedBody))
	require.Equal(t, len(logs[0].Topics), 4)
	require.True(t, 0 == bytes.Compare(logs[0].Topics[0], []byte(testEvents[0].Topics[0])))

	require.NoError(t, receiptHandler.Close())
}
