(set-option :rlimit 1000000)
(declare-fun p (Int) Bool)(declare-const a Int)(declare-const b Int)(assert (forall ((x Int)) (=> (distinct (+ b (mod 4 5)) a) (and (=> (<= a (+ b (- b 8))) (< (+ (ite (= b b) a x) (+ a b)) b)) (and (>= (* 6 (mod b 4)) 6) (<= (div (+ b x) 4) (+ (+ b b) (+ b a))))))))(apply qe2)
