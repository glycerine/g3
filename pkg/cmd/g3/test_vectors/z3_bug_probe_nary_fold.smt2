(check-sat-using (if (= (+ 1 2) 4) (fail-if 1) smt))
(check-sat-using (if (= (* 2 3) 12) (fail-if 1) smt))
(check-sat-using (if (= (+ 1 2) 3) (fail-if 1) smt))
(check-sat-using (if (= (+ 1 2 4) 7) (fail-if 1) smt))
