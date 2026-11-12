(declare-datatypes (T) ((List (nil) (cons (hd T) (tl List)))))
(declare-const l (List Int))
(check-sat)
