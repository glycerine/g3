(push)(define-fun-rec f ((x Int)) Int x)(pop)(assert (= (f 1) 2))(check-sat)
