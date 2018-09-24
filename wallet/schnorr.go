package wallet

import (
	"gopkg.in/dedis/kyber.v2"
	"gopkg.in/dedis/kyber.v2/group/edwards25519"
)

var curve = edwards25519.NewBlakeSHA256Ed25519()
var hashSha256 = curve.Hash()
var g = curve.Point().Base()

// type Signature struct {
// 	r kyber.Point
// 	s kyber.Scalar
// }

func Hash(s string) kyber.Scalar {
	hashSha256.Reset()
	hashSha256.Write([]byte(s))
	return curve.Scalar().SetBytes(hashSha256.Sum(nil))
}

/*
	both generate `k`
	both do r = k*G, r1 e r2
	after some them up r, r1 + r2
	P = (publickey1 + publickey2)
	e = m + (r1+r2) + P
	s = k – e * x , s1 e s2
	e S = s1 + s2
	The verification by checking that R = s * G + H(m || P || R) * P


	C = H(P0 || P1)
	Q0 = H(C || P0) * P0 , Q1 = H(C || P1) * P1
	P = Q0 + Q1
	Alice uses y0 = x0 * H(C || P0) as private key , Bob y1 = x1 * H(C || P1)
*/

// m: Message
// x: Private key
func Sign(m string, x kyber.Scalar, otherR []kyber.Point, otherP []kyber.Point, k kyber.Scalar) kyber.Scalar {
	// SHARD THIS
	// r = k * G
	myR := curve.Point().Mul(k, g)
	// p = x * G
	myP := curve.Point().Mul(x, g)

	R := myR
	for _, r := range otherR {
		R = curve.Point().Add(R, r)
	}
	P := myP
	for _, p := range otherP {
		P = curve.Point().Add(P, p)
	}

	// C := Hash(P.String())
	// myQ := curve.Point().Mul(Hash(C.String()+myP.String()), myP)
	// P2 := myQ
	// for _, p := range otherP {
	// 	P2 = curve.Point().Add(P2, curve.Point().Mul(Hash(C.String()+p.String()), p))
	// }
	// e := Hash(m + P2.String() + R.String())

	// Hash(m || r || p)
	e := Hash(m + P.String() + R.String())

	// s = k - e * x
	s := curve.Scalar().Sub(k, curve.Scalar().Mul(e, x))
	return s
}

func PublicKey(m string, rSignature kyber.Point, sSignature kyber.Scalar) kyber.Point {
	// e = Hash(m || r)
	e := Hash(m + rSignature.String())

	// y = (r - s * G) * (1 / e)
	y := curve.Point().Sub(rSignature, curve.Point().Mul(sSignature, g))
	y = curve.Point().Mul(curve.Scalar().Div(curve.Scalar().One(), e), y)

	return y
}

func Verify(m string, rSignature kyber.Point, sSignature kyber.Scalar, P kyber.Point, R kyber.Point) bool {
	// e = Hash(m || r || P)
	e := Hash(m + P.String() + R.String())

	// check R = s * G + H(m || P || R) * P
	a := curve.Point().Add(curve.Point().Mul(sSignature, g), curve.Point().Mul(e, P))
	return R.Equal(a)
}

// func (S Signature) String() string {
// 	return fmt.Sprintf("(r=%s, s=%s)", S.r, S.s)
// }

// func ByteToPoint(byteRs [][]byte, Ps []string) []kyber.Point {
// var Rs []kyber.Point
// for _, r := range byteRs {
// 	var byteR bytes.Buffer
// 	dec := gob.NewDecoder(&byteR)
// 	err := dec.Decode(&r)
// 	if err != nil {
// 		return nil
// 	}
// 	Rs = append(Rs, r)
// }
// return Rs
// }

// func MarshalGob(v interface{}) ([]byte, error) {
// 	b := new(bytes.Buffer)
// 	err := gob.NewEncoder(b).Encode(v)
// 	if err != nil {
// 		fmt.Println("ERROR ", err)
// 		return nil, err
// 	}
// 	return b.Bytes(), nil
// }

// func UnmarshalGob(data []byte, v *kyber.Point) error {
// 	b := bytes.NewBuffer(data)
// 	return gob.NewDecoder(b).Decode(&v)
// }

// func SignatureToByte(S Signature) ([]byte, error) {
// 	sByte, err := MarshalGob(S)
// 	return sByte, err
// }

func ByteToPoint(b []byte) (kyber.Point, error) {
	p := curve.Point()
	err := p.UnmarshalBinary(b)
	if err != nil {
		return nil, err
	}
	return p, nil
}

func ByteToScalar(b []byte) (kyber.Scalar, error) {
	p := curve.Scalar()
	err := p.UnmarshalBinary(b)
	if err != nil {
		return nil, err
	}
	return p, nil
}

// generate k and calculate r
func GenerateParameter() (kyber.Scalar, []byte, error) {
	k := curve.Scalar().Pick(curve.RandomStream())
	r := curve.Point().Mul(k, g)

	res, err := r.MarshalBinary()
	if err != nil {
		return nil, nil, err
	}
	return k, res, nil
}

func MakeSign(x kyber.Scalar, k kyber.Scalar, message string, otherR []kyber.Point, otherP []kyber.Point) kyber.Scalar {
	return Sign(message, x, otherR, otherP, k)
}

func CreateSignature(Rs []kyber.Point, myR kyber.Point, Ss []kyber.Scalar) ([]byte, []byte, error) {
	R := myR
	for _, r := range Rs {
		R = curve.Point().Add(R, r)
	}

	S := Ss[0]
	for _, s := range Ss {
		S = curve.Scalar().Add(S, s)
	}

	byteR, err := R.MarshalBinary()
	if err != nil {
		return nil, nil, err
	}

	byteS, err := S.MarshalBinary()
	if err != nil {
		return nil, nil, err
	}

	return byteR, byteS, nil
}

func VerifySignature(message string, rSignature kyber.Point, sSignature kyber.Scalar, otherP []kyber.Point, myP kyber.Point, otherR []kyber.Point, myR kyber.Point) bool {
	P := myP
	for _, p := range otherP {
		P = curve.Point().Add(P, p)
	}

	R := myR
	for _, r := range otherR {
		R = curve.Point().Add(R, r)
	}

	v := Verify(message, rSignature, sSignature, P, R)
	return v
}

// return x, p
func CreateSchnorrKeys() (kyber.Scalar, kyber.Point) {
	privateKey := curve.Scalar().Pick(curve.RandomStream())
	publicKey := curve.Point().Mul(privateKey, curve.Point().Base())
	return privateKey, publicKey
}
