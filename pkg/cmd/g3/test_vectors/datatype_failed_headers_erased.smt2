(declare-datatypes ((A 0)) (((a (x B)))))(declare-fun p (A) Bool)(declare-const y A)(assert (p y))(check-sat)
(declare-sort A)
(declare-datatypes ((C 0) (A 0)) (((c)) ((a (x B)))))(declare-const w C)
(echo "end")
