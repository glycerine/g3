(push)(define-fun-rec f ((x Int)) Int x)(pop)(declare-fun f (Int) Int)(assert (= (f 1) 2))(check-sat)
