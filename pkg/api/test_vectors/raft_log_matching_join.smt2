; Isolated normalized Raft log_matching axiom and its three-term trigger.
; All ground witnesses and finite domains are defined here; no Ivy files needed.
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


(declare-fun term_at ((_ BitVec 4) (_ BitVec 3) (_ BitVec 3)) Bool)
(declare-fun value_at ((_ BitVec 4) (_ BitVec 3)) (_ BitVec 2))
(declare-fun valid ((_ BitVec 4)) Bool)
(declare-fun index_used ((_ BitVec 4) (_ BitVec 3)) Bool)
(assert (forall ((|I1:index| (_ BitVec 3))
         (|I2:index| (_ BitVec 3))
         (|L1:log| (_ BitVec 4))
         (|L2:log| (_ BitVec 4))
         (|T1:term| (_ BitVec 3))
         (|T2:term| (_ BitVec 3)))
  (! (let ((a!1 (or (= (not (term_at |L1:log| |I2:index| |T2:term|))
                       (term_at |L2:log| |I2:index| |T2:term|))
                    (not (= (value_at |L1:log| |I2:index|)
                            (value_at |L2:log| |I2:index|))))))
       (or (not (valid |L1:log|))
           (not (valid |L2:log|))
           (not (index_used |L1:log| |I1:index|))
           (not (term_at |L1:log| |I1:index| |T1:term|))
           (not (index_used |L2:log| |I1:index|))
           (not (term_at |L2:log| |I1:index| |T1:term|))
           (not (bvule |I2:index| |I1:index|))
           (not a!1)))
     :pattern ((term_at |L1:log| |I1:index| |T1:term|)
               (term_at |L2:log| |I1:index| |T1:term|)
               (term_at |L1:log| |I2:index| |T2:term|)))))
(assert (valid (_ bv0 4)))
(assert (valid (_ bv1 4)))
(assert (index_used (_ bv0 4) (_ bv1 3)))
(assert (index_used (_ bv1 4) (_ bv1 3)))
(assert (term_at (_ bv0 4) (_ bv1 3) (_ bv1 3)))
(assert (term_at (_ bv1 4) (_ bv1 3) (_ bv1 3)))
(assert (term_at (_ bv0 4) (_ bv0 3) (_ bv0 3)))
(assert (not (term_at (_ bv1 4) (_ bv0 3) (_ bv0 3))))
(check-sat)
