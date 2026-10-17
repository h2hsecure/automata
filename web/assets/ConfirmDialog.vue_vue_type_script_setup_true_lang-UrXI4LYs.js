import{c as u}from"./createLucideIcon-ouXDw_n7.js";import{d as C,l as $,c as p,w as B,a as o,t as r,b as h,u as s,B as x,e as _,T,o as d,r as P,C as V,f as w,h as g}from"./index-DuVkRjGg.js";/**
 * @license lucide-vue-next v0.400.0 - ISC
 *
 * This source code is licensed under the ISC license.
 * See the LICENSE file in the root directory of this source tree.
 */const K=u("PencilIcon",[["path",{d:"M21.174 6.812a1 1 0 0 0-3.986-3.987L3.842 16.174a2 2 0 0 0-.5.83l-1.321 4.352a.5.5 0 0 0 .623.622l4.353-1.32a2 2 0 0 0 .83-.497z",key:"1a8usu"}],["path",{d:"m15 5 4 4",key:"1mk7zo"}]]);/**
 * @license lucide-vue-next v0.400.0 - ISC
 *
 * This source code is licensed under the ISC license.
 * See the LICENSE file in the root directory of this source tree.
 */const O=u("PlusIcon",[["path",{d:"M5 12h14",key:"1ays0h"}],["path",{d:"M12 5v14",key:"s699le"}]]);/**
 * @license lucide-vue-next v0.400.0 - ISC
 *
 * This source code is licensed under the ISC license.
 * See the LICENSE file in the root directory of this source tree.
 */const Q=u("Trash2Icon",[["path",{d:"M3 6h18",key:"d0wm0j"}],["path",{d:"M19 6v14c0 1-1 2-2 2H7c-1 0-2-1-2-2V6",key:"4alrt4"}],["path",{d:"M8 6V4c0-1 1-2 2-2h4c1 0 2 1 2 2v2",key:"v07s0e"}],["line",{x1:"10",x2:"10",y1:"11",y2:"17",key:"1uufr5"}],["line",{x1:"14",x2:"14",y1:"11",y2:"17",key:"xtxkd"}]]);/**
 * @license lucide-vue-next v0.400.0 - ISC
 *
 * This source code is licensed under the ISC license.
 * See the LICENSE file in the root directory of this source tree.
 */const j=u("XIcon",[["path",{d:"M18 6 6 18",key:"1bl5f8"}],["path",{d:"m6 6 12 12",key:"d8bk6v"}]]),I={class:"w-full max-w-lg rounded-lg border bg-card shadow-lg"},z={class:"flex items-center justify-between border-b p-4"},N={class:"text-lg font-semibold"},S={class:"p-4"},D={key:0,class:"border-t p-4"},U=C({__name:"Modal",props:{open:{type:Boolean},title:{}},emits:["close"],setup(l,{emit:c}){const e=c;return(n,t)=>(d(),$(T,{to:"body"},[l.open?(d(),p("div",{key:0,class:"fixed inset-0 z-50 flex items-center justify-center bg-black/50",onClick:t[1]||(t[1]=B(m=>e("close"),["self"]))},[o("div",I,[o("div",z,[o("h3",N,r(l.title),1),o("button",{class:"rounded p-1 hover:bg-muted",onClick:t[0]||(t[0]=m=>e("close"))},[h(s(j),{class:"h-4 w-4"})])]),o("div",S,[x(n.$slots,"default")]),n.$slots.footer?(d(),p("div",D,[x(n.$slots,"footer")])):_("",!0)])])):_("",!0)]))}}),f=P({show:!1,options:null,resolve:null});let a=null;function L(){function l(n){return new Promise(t=>{a=t,f.value={show:!0,options:{confirmText:"Confirm",cancelText:"Cancel",...n},resolve:t}})}function c(){a&&(a(!0),a=null),f.value.show=!1}function e(){a&&(a(!1),a=null),f.value.show=!1}return{confirmStore:f,show:l,confirm:c,cancel:e}}const X={key:0,class:"fixed inset-0 z-50 flex items-center justify-center bg-black/50"},E={class:"w-full max-w-md rounded-lg border bg-card p-6 shadow-lg"},H={class:"text-lg font-semibold mb-2"},R={class:"text-sm text-muted-foreground mb-6"},q={class:"flex justify-end gap-2"},W=C({__name:"ConfirmDialog",emits:["confirm"],setup(l,{emit:c}){const{confirmStore:e,confirm:n,cancel:t}=L(),m=c;async function M(){n(),m("confirm")}return(A,F)=>{var k,v,b;const y=V("Button");return d(),$(T,{to:"body"},[s(e).show?(d(),p("div",X,[o("div",E,[o("h3",H,r((k=s(e).options)==null?void 0:k.title),1),o("p",R,r((v=s(e).options)==null?void 0:v.description),1),o("div",q,[h(y,{variant:"outline",onClick:s(t)},{default:w(()=>{var i;return[g(r(((i=s(e).options)==null?void 0:i.cancelText)||"Cancel"),1)]}),_:1},8,["onClick"]),h(y,{variant:((b=s(e).options)==null?void 0:b.variant)||"default",onClick:M},{default:w(()=>{var i;return[g(r(((i=s(e).options)==null?void 0:i.confirmText)||"Confirm"),1)]}),_:1},8,["variant"])])])])):_("",!0)])}}});export{O as P,Q as T,j as X,U as _,K as a,W as b,L as u};
