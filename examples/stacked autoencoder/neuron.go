package main

import (
	"bytes"
	"encoding/gob"
	"errors"

	. "github.com/chewxy/gorgonia"
	tf64 "github.com/chewxy/gorgonia/tensor/f64"
)

type Neuron struct {
	w *Node
	b *Node

	g *ExprGraph
}

type initFn func() InitWFn

func NewNeuron(inputs, outputs, batchSize int, g *ExprGraph, fn initFn) *Neuron {
	w := NewMatrix(g, Float64, WithShape(inputs, outputs), WithInit(fn()))

	var b *Node
	if batchSize == 1 {
		b = NewVector(g, Float64, WithShape(outputs), WithInit(Zeroes()))
	} else {
		b = NewMatrix(g, Float64, WithShape(batchSize, outputs), WithInit(Zeroes()))
	}

	return &Neuron{
		w: w,
		b: b,
		g: g,
	}

}

func (n *Neuron) GobEncode() (p []byte, err error) {
	// check first
	if n.w.Value() == nil {
		err = errors.New("Cannot encode nil `w`")
		return
	}

	if n.b.Value() == nil {
		err = errors.New("Cannot encode nil `b`")
		return
	}

	var wge, bge gob.GobEncoder
	var ok bool
	if wge, ok = n.w.Value().(Tensor).Tensor.(gob.GobEncoder); !ok {
		err = errors.New("Cannot encode non-GobEncoder `w`") // well you technically can but we're adding an extra restriction here
		return
	}

	if bge, ok = n.b.Value().(Tensor).Tensor.(gob.GobEncoder); !ok {
		err = errors.New("Cannot encode non-GobEncoder `b`")
		return
	}

	var buf bytes.Buffer
	encoder := gob.NewEncoder(&buf)
	if err = encoder.Encode(wge); err != nil {
		return
	}

	if err = encoder.Encode(bge); err != nil {
		return
	}

	p = buf.Bytes()
	return
}

func (n *Neuron) GobDecode(p []byte) (err error) {
	if n == nil {
		err = errors.New("Cannot decode nil Neuron")
		return
	}

	if n.w == nil {
		err = errors.New("Cannot decode nil `w`")
		return
	}

	if n.b == nil {
		err = errors.New("Cannot decode nil `b`")
		return
	}

	buf := bytes.NewBuffer(p)
	decoder := gob.NewDecoder(buf)

	var wT, bT *tf64.Tensor
	if err = decoder.Decode(&wT); err != nil {
		return
	}

	if err = decoder.Decode(&bT); err != nil {
		return
	}

	Let(n.w, wT)
	Let(n.b, bT)

	return
}
