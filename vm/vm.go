package vm

import (
	"errors"

	"github.com/loomnetwork/loom"
	loom "github.com/loomnetwork/loom-plugin"
)

type VM interface {
	Create(caller loom.Address, code []byte) ([]byte, loom.Address, error)
	Call(caller, addr loom.Address, input []byte) ([]byte, error)
	StaticCall(caller, addr loom.Address, input []byte) ([]byte, error)
}

type Factory func(loomchain.State) VM

type Manager struct {
	vms map[VMType]Factory
}

func NewManager() *Manager {
	return &Manager{
		vms: make(map[VMType]Factory),
	}
}

func (m *Manager) Register(typ VMType, fac Factory) {
	m.vms[typ] = fac
}

func (m *Manager) InitVM(typ VMType, state loomchain.State) (VM, error) {
	fac, ok := m.vms[typ]
	if !ok {
		return nil, errors.New("vm type not found")
	}

	return fac(state), nil
}
