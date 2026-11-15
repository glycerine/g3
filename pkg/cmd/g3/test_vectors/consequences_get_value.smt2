(set-option :produce-models true)(declare-const x Int)(declare-const b Bool)(assert (=> b (> x 3)))(assert b)(get-consequences (b) (b))(get-value (x b))(eval (+ x 1))(get-info :reason-unknown)
