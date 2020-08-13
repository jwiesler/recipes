package main

import (
	"errors"
	"strings"
)

type Unit struct {
	Identifier string
}

type UnitsRegistry struct {
	units map[string]*Unit
}

func (r *UnitsRegistry) Register(u string) *Unit {
	unit := &Unit{Identifier: u}
	r.RegisterUnit(unit)
	return unit
}

func (r *UnitsRegistry) RegisterUnit(unit *Unit) {
	r.units[unit.Identifier] = unit
}

func (r *UnitsRegistry) GetOrRegister(u string) *Unit {
	unit, ok := r.units[u]
	if !ok {
		return r.Register(u)
	}
	return unit
}

var (
	unitEL = Unit{"EL"}
	unitTL = Unit{"TL"}
	unitG  = Unit{"g"}
	unitML = Unit{"ml"}
)

func DefaultRegistry() UnitsRegistry {
	reg := UnitsRegistry{
		units: make(map[string]*Unit),
	}
	reg.RegisterUnit(&unitEL)
	reg.RegisterUnit(&unitTL)
	reg.RegisterUnit(&unitG)
	reg.RegisterUnit(&unitML)
	return reg
}

func (u *Unit) UnmarshalText(text []byte) error {
	switch strings.ToLower(string(text)) {
	default:
		return errors.New("unrecognized unit")
	case "el":
		*u = unitEL
	case "tl":
		*u = unitTL
	case "g":
		*u = unitG
	case "ml":
		*u = unitML
	}
	return nil
}

func (u *Unit) MarshalText() ([]byte, error) {
	return []byte(u.Identifier), nil
}
