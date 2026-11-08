package ast

// ListElem is the element type the checker knows for the list argument of a TR_sum or TR_mean
// call (CallExpression.ListElem, written by the checker's noteListElem). An empty list's own
// elements cannot tell its type; the run time takes it from here: an empty tensor list's sum is
// an error (15.18). It is ListElemUnknown when the source was not checked or the type is not known.
type ListElem uint8

const (
	ListElemUnknown ListElem = iota
	ListElemTensor           // TR_List[TR_Tensor]
)
