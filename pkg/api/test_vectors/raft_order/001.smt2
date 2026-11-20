; Verification condition 1 emitted by Ivy for examples/raft/raft_no_assume_test.ivy.
; Copyright (c) 2025 James R. Wilcox
;
; Permission is hereby granted, free of charge,
; to any person obtaining a copy of this software and
; associated documentation files (the "Software"),
; to deal in the Software without restriction,
; including without limitation the rights to use,
; copy, modify, merge, publish, distribute, sublicense,
; and/or sell copies of the Software, and to permit
; persons to whom the Software is furnished to do so,
; subject to the following conditions:
;
; The above copyright notice and this permission notice
; shall be included in all copies or substantial
; portions of the Software.
;
; THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY
; OF ANY KIND, EXPRESS OR IMPLIED, INCLUDING BUT NOT
; LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
; FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT.
; IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS
; BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY,
; WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE,
; ARISING FROM, OUT OF OR IN CONNECTION WITH THE
; SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.

(declare-fun term_bigger ((_ BitVec 2) (_ BitVec 3)) Bool)
(declare-fun append_reply_msg
             ((_ BitVec 2) (_ BitVec 3) (_ BitVec 3) (_ BitVec 3))
             Bool)
(declare-fun member ((_ BitVec 2) (_ BitVec 4)) Bool)
(declare-fun choosable ((_ BitVec 3) (_ BitVec 3)) Bool)
(declare-fun log0 () (_ BitVec 4))
(declare-fun valid ((_ BitVec 4)) Bool)
(declare-fun term_log ((_ BitVec 3)) (_ BitVec 4))
(declare-fun commit_log () (_ BitVec 4))
(declare-fun commit_index () (_ BitVec 3))
(declare-fun committed ((_ BitVec 3) (_ BitVec 2)) Bool)
(declare-fun vote_msg ((_ BitVec 2) (_ BitVec 3) (_ BitVec 2)) Bool)
(declare-fun request_vote_msg
             ((_ BitVec 2) (_ BitVec 3) (_ BitVec 4) (_ BitVec 4))
             Bool)
(declare-fun __append_reply_msg
             ((_ BitVec 2) (_ BitVec 3) (_ BitVec 3) (_ BitVec 3))
             Bool)
(declare-fun __m_choosable ((_ BitVec 3) (_ BitVec 3)) Bool)
(declare-fun append_msg
             ((_ BitVec 3)
              (_ BitVec 4)
              (_ BitVec 4)
              (_ BitVec 3)
              (_ BitVec 3)
              (_ BitVec 2))
             Bool)
(declare-fun term_at ((_ BitVec 4) (_ BitVec 3) (_ BitVec 3)) Bool)
(declare-fun index_used ((_ BitVec 4) (_ BitVec 3)) Bool)
(declare-fun current_log ((_ BitVec 2)) (_ BitVec 4))
(declare-fun current_term ((_ BitVec 2) (_ BitVec 3)) Bool)
(declare-fun voted ((_ BitVec 2) (_ BitVec 3)) Bool)
(declare-fun started ((_ BitVec 3)) Bool)
(declare-fun leader ((_ BitVec 2) (_ BitVec 3)) Bool)
(declare-fun __term_bigger ((_ BitVec 2) (_ BitVec 3)) Bool)
(declare-fun __choosable ((_ BitVec 3) (_ BitVec 3)) Bool)
(declare-fun none_term () (_ BitVec 4))
(declare-fun is_none_term ((_ BitVec 4)) Bool)
(declare-fun none_index () (_ BitVec 4))
(declare-fun is_none_index ((_ BitVec 4)) Bool)
(declare-fun @T () (_ BitVec 3))
(declare-fun @N1 () (_ BitVec 2))
(declare-fun @N2 () (_ BitVec 2))
(assert (let ((a!1 (forall ((|X:term| (_ BitVec 3))
                    (|Y:term| (_ BitVec 3))
                    (|Z:term| (_ BitVec 3)))
             (=> (and (bvule |X:term| |Y:term|) (bvule |Y:term| |Z:term|))
                 (bvule |X:term| |Z:term|))))
      (a!2 (forall ((|X:term| (_ BitVec 3)) (|Y:term| (_ BitVec 3)))
             (=> (and (bvule |X:term| |Y:term|) (bvule |Y:term| |X:term|))
                 (= |X:term| |Y:term|))))
      (a!3 (forall ((|X:term| (_ BitVec 3)) (|Y:term| (_ BitVec 3)))
             (= (bvult |X:term| |Y:term|)
                (and (bvule |X:term| |Y:term|) (not (= |X:term| |Y:term|))))))
      (a!4 (forall ((|X:index| (_ BitVec 3))
                    (|Y:index| (_ BitVec 3))
                    (|Z:index| (_ BitVec 3)))
             (=> (and (bvule |X:index| |Y:index|) (bvule |Y:index| |Z:index|))
                 (bvule |X:index| |Z:index|))))
      (a!5 (forall ((|X:index| (_ BitVec 3)) (|Y:index| (_ BitVec 3)))
             (=> (and (bvule |X:index| |Y:index|) (bvule |Y:index| |X:index|))
                 (= |X:index| |Y:index|))))
      (a!6 (forall ((|X:index| (_ BitVec 3)) (|Y:index| (_ BitVec 3)))
             (= (bvult |X:index| |Y:index|)
                (and (bvule |X:index| |Y:index|) (not (= |X:index| |Y:index|))))))
      (a!7 (forall ((|Q1:quorum| (_ BitVec 4)) (|Q2:quorum| (_ BitVec 4)))
             (exists ((|N:node| (_ BitVec 2)))
               (and (member |N:node| |Q1:quorum|) (member |N:node| |Q2:quorum|))))))
  (and (forall ((|X:term| (_ BitVec 3))) (bvule |X:term| |X:term|))
       a!1
       a!2
       (forall ((|X:term| (_ BitVec 3)) (|Y:term| (_ BitVec 3)))
         (or (bvule |X:term| |Y:term|) (bvule |Y:term| |X:term|)))
       (forall ((|X:term| (_ BitVec 3))) (bvule #b000 |X:term|))
       a!3
       (forall ((|X:term| (_ BitVec 3)) (|Y:term| (_ BitVec 3)))
         (= (bvugt |X:term| |Y:term|) (bvult |Y:term| |X:term|)))
       (forall ((|X:term| (_ BitVec 3)) (|Y:term| (_ BitVec 3)))
         (= (bvuge |X:term| |Y:term|) (bvule |Y:term| |X:term|)))
       (forall ((|X:index| (_ BitVec 3))) (bvule |X:index| |X:index|))
       a!4
       a!5
       (forall ((|X:index| (_ BitVec 3)) (|Y:index| (_ BitVec 3)))
         (or (bvule |X:index| |Y:index|) (bvule |Y:index| |X:index|)))
       (forall ((|X:index| (_ BitVec 3))) (bvule #b000 |X:index|))
       a!6
       (forall ((|X:index| (_ BitVec 3)) (|Y:index| (_ BitVec 3)))
         (= (bvugt |X:index| |Y:index|) (bvult |Y:index| |X:index|)))
       (forall ((|X:index| (_ BitVec 3)) (|Y:index| (_ BitVec 3)))
         (= (bvuge |X:index| |Y:index|) (bvule |Y:index| |X:index|)))
       a!7
       (forall ((|OI:option_index| (_ BitVec 4)))
         (= (is_none_index |OI:option_index|) (= |OI:option_index| none_index)))
       (forall ((|OT:option_term| (_ BitVec 4)))
         (= (is_none_term |OT:option_term|) (= |OT:option_term| none_term)))
       (forall ((|I:index| (_ BitVec 3)) (|T:term| (_ BitVec 3)))
         (let ((a!1 (exists ((|Q:quorum| (_ BitVec 4)))
                      (forall ((|N:node| (_ BitVec 2)))
                        (=> (member |N:node| |Q:quorum|)
                            (or (__append_reply_msg
                                  |N:node|
                                  |T:term|
                                  |I:index|
                                  |T:term|)
                                (not (__term_bigger |N:node| |T:term|))))))))
           (= (__choosable |I:index| |T:term|) a!1)))
       (forall ((|V0:node| (_ BitVec 2)) (|V1:term| (_ BitVec 3)))
         (not (leader |V0:node| |V1:term|)))
       (forall ((|V0:term| (_ BitVec 3))) (not (started |V0:term|)))
       (forall ((|V0:node| (_ BitVec 2)) (|V1:term| (_ BitVec 3)))
         (not (voted |V0:node| |V1:term|)))
       (forall ((|V0:node| (_ BitVec 2)) (|V1:term| (_ BitVec 3)))
         (= (current_term |V0:node| |V1:term|) (= |V1:term| #b000)))
       (forall ((|V0:node| (_ BitVec 2)) (|V1:term| (_ BitVec 3)))
         (not (term_bigger |V0:node| |V1:term|)))
       (forall ((|V0:node| (_ BitVec 2))) (= (current_log |V0:node|) log0))
       (forall ((|V0:log| (_ BitVec 4)) (|V1:index| (_ BitVec 3)))
         (not (index_used |V0:log| |V1:index|)))
       (forall ((|V0:log| (_ BitVec 4))
                (|V1:index| (_ BitVec 3))
                (|V2:term| (_ BitVec 3)))
         (not (term_at |V0:log| |V1:index| |V2:term|)))
       (forall ((|V0:term| (_ BitVec 3))
                (|V1:option_index| (_ BitVec 4))
                (|V2:option_term| (_ BitVec 4))
                (|V3:index| (_ BitVec 3))
                (|V4:term| (_ BitVec 3))
                (|V5:value| (_ BitVec 2)))
         (not (append_msg |V0:term|
                          |V1:option_index|
                          |V2:option_term|
                          |V3:index|
                          |V4:term|
                          |V5:value|)))
       (forall ((|V0:node| (_ BitVec 2))
                (|V1:term| (_ BitVec 3))
                (|V2:index| (_ BitVec 3))
                (|V3:term| (_ BitVec 3)))
         (not (append_reply_msg |V0:node| |V1:term| |V2:index| |V3:term|)))
       (forall ((|I:index| (_ BitVec 3)) (|T:term| (_ BitVec 3)))
         (let ((a!1 (exists ((|Q:quorum| (_ BitVec 4)))
                      (forall ((|N:node| (_ BitVec 2)))
                        (=> (member |N:node| |Q:quorum|)
                            (or (__append_reply_msg
                                  |N:node|
                                  |T:term|
                                  |I:index|
                                  |T:term|)
                                (not (term_bigger |N:node| |T:term|))))))))
           (= (__m_choosable |I:index| |T:term|) a!1)))
       (forall ((|V0:node| (_ BitVec 2))
                (|V1:term| (_ BitVec 3))
                (|V2:option_index| (_ BitVec 4))
                (|V3:option_term| (_ BitVec 4)))
         (not (request_vote_msg |V0:node|
                                |V1:term|
                                |V2:option_index|
                                |V3:option_term|)))
       (forall ((|V0:node| (_ BitVec 2))
                (|V1:term| (_ BitVec 3))
                (|V2:node| (_ BitVec 2)))
         (not (vote_msg |V0:node| |V1:term| |V2:node|)))
       (forall ((|V0:index| (_ BitVec 3)) (|V1:value| (_ BitVec 2)))
         (not (committed |V0:index| |V1:value|)))
       (= commit_index #b000)
       (= commit_log log0)
       (forall ((|V0:term| (_ BitVec 3))) (= (term_log |V0:term|) log0))
       (forall ((|V0:log| (_ BitVec 4))) (= (valid |V0:log|) (= |V0:log| log0)))
       (forall ((|I:index| (_ BitVec 3)) (|T:term| (_ BitVec 3)))
         (let ((a!1 (exists ((|Q:quorum| (_ BitVec 4)))
                      (forall ((|N:node| (_ BitVec 2)))
                        (=> (member |N:node| |Q:quorum|)
                            (or (append_reply_msg |N:node|
                                                  |T:term|
                                                  |I:index|
                                                  |T:term|)
                                (not (term_bigger |N:node| |T:term|))))))))
           (= (choosable |I:index| |T:term|) a!1))))))
(assert (and (not (=> (vote_msg @N2 @T @N1) (voted @N1 @T)))))
