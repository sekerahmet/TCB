package gpu

import "fmt"

// simNaNProp min/max'ın .NaN eki (mode2): girdilerden biri NaN ise sonuç kanonik NaN.
const simNaNProp = 6

// extras komutun nokta eklerini (tür, kip, alan) çözer.
func (a *simParser) extras(k *simInst, ek []string) error {
	var dtypes []simType
	for _, e := range ek {
		if t, ok := simTypeNames[e]; ok {
			dtypes = append(dtypes, t)
			continue
		}
		switch e {
		case "lo":
			k.mode2 = 1
		case "wide":
			if k.op == simMul {
				k.op = simMulWide
			} else {
				k.op = simMadWide
			}
		case "NaN":
			k.mode2 = simNaNProp // min/max: NaN'ı korur
		case "rn", "approx", "uni", "sync", "to", "async", "ca", "cg":
		case "v2":
			k.vec = 2
		case "v4":
			k.vec = 4
		case "commit_group":
			k.op = simCpGroup
		case "wait_group":
			k.op = simCpWait
		case "rni":
			k.mode2 = 1
		case "rzi":
			k.mode2 = 2
		case "rmi":
			k.mode2 = 3
		case "rpi":
			k.mode2 = 4
		case "param":
			k.space = spaceParam
		case "global":
			k.space = spaceGlobal
		case "shared":
			k.space = spaceShared
		default:
			kp, ok := simCompare[e]
			if !ok || k.op != simSetp {
				return fmt.Errorf("benzetimin bilmediği ek .%s", e)
			}
			k.mode2 = kp
		}
	}
	if len(dtypes) > 0 {
		k.dtype = dtypes[0]
	}
	if len(dtypes) > 1 {
		k.source = dtypes[1]
	}
	if (k.op == simMul || k.op == simMad) && k.dtype != simF32 && k.dtype != simF64 && k.mode2 != 1 {
		return fmt.Errorf("tam sayı çarpımı .lo ya da .wide ister")
	}
	return nil
}
