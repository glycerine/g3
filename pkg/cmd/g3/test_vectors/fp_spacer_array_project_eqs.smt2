; AUDIT.md qe_03-F4: spacer reach facts project the pre-state array a out
; of (= b (store a i 0)) through array_project_eqs_util. Compare its trace
; with G3_GOLDEN_TAGS=qe,model_evaluator.
(set-option :fixedpoint.engine spacer)
(declare-rel Inv ((Array Int Int) Int))
(declare-rel Err ())
(declare-var a (Array Int Int))
(declare-var b (Array Int Int))
(declare-var i Int)
(rule (=> (= i 0) (Inv a i)))
(rule (=> (and (Inv a i) (< i 10) (= b (store a i 0))) (Inv b (+ i 1))))
(rule (=> (and (Inv a i) (>= i 3) (= (select a 1) 0)) Err))
(query Err :print-certificate true)
