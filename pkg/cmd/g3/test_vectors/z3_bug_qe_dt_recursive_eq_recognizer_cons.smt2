(declare-datatypes ((Lst 0)) (((cons (hd Int) (tl Lst)) (nil))))
(declare-const y Lst)
(assert (exists ((x Lst)) (and ((_ is cons) x) (= (cons 1 x) y))))
(apply qe)
