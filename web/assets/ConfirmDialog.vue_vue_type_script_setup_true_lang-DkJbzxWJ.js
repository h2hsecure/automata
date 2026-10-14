import{c as f}from"./createLucideIcon-DEZk1XcL.js";import{r as w,d as b,q as g,o as _,m as T,u as t,c as B,a,t as c,b as x,f as y,h as k,e as M,T as V}from"./index-D5EP8eDN.js";/**
 * @license lucide-vue-next v0.400.0 - ISC
 *
 * This source code is licensed under the ISC license.
 * See the LICENSE file in the root directory of this source tree.
 */const H=f("PlusIcon",[["path",{d:"M5 12h14",key:"1ays0h"}],["path",{d:"M12 5v14",key:"s699le"}]]);/**
 * @license lucide-vue-next v0.400.0 - ISC
 *
 * This source code is licensed under the ISC license.
 * See the LICENSE file in the root directory of this source tree.
 */const L=f("Trash2Icon",[["path",{d:"M3 6h18",key:"d0wm0j"}],["path",{d:"M19 6v14c0 1-1 2-2 2H7c-1 0-2-1-2-2V6",key:"4alrt4"}],["path",{d:"M8 6V4c0-1 1-2 2-2h4c1 0 2 1 2 2v2",key:"v07s0e"}],["line",{x1:"10",x2:"10",y1:"11",y2:"17",key:"1uufr5"}],["line",{x1:"14",x2:"14",y1:"11",y2:"17",key:"xtxkd"}]]);/**
 * @license lucide-vue-next v0.400.0 - ISC
 *
 * This source code is licensed under the ISC license.
 * See the LICENSE file in the root directory of this source tree.
 */const R=f("XIcon",[["path",{d:"M18 6 6 18",key:"1bl5f8"}],["path",{d:"m6 6 12 12",key:"d8bk6v"}]]),i=w({show:!1,options:null,resolve:null});let o=null;function I(){function d(r){return new Promise(s=>{o=s,i.value={show:!0,options:{confirmText:"Confirm",cancelText:"Cancel",...r},resolve:s}})}function l(){o&&(o(!0),o=null),i.value.show=!1}function e(){o&&(o(!1),o=null),i.value.show=!1}return{confirmStore:i,show:d,confirm:l,cancel:e}}const N={key:0,class:"fixed inset-0 z-50 flex items-center justify-center bg-black/50"},P={class:"w-full max-w-md rounded-lg border bg-card p-6 shadow-lg"},j={class:"text-lg font-semibold mb-2"},S={class:"text-sm text-muted-foreground mb-6"},D={class:"flex justify-end gap-2"},A=b({__name:"ConfirmDialog",emits:["confirm"],setup(d,{emit:l}){const{confirmStore:e,confirm:r,cancel:s}=I(),v=l;async function C(){r(),v("confirm")}return(X,q)=>{var m,p,h;const u=g("Button");return _(),T(V,{to:"body"},[t(e).show?(_(),B("div",N,[a("div",P,[a("h3",j,c((m=t(e).options)==null?void 0:m.title),1),a("p",S,c((p=t(e).options)==null?void 0:p.description),1),a("div",D,[x(u,{variant:"outline",onClick:t(s)},{default:y(()=>{var n;return[k(c(((n=t(e).options)==null?void 0:n.cancelText)||"Cancel"),1)]}),_:1},8,["onClick"]),x(u,{variant:((h=t(e).options)==null?void 0:h.variant)||"default",onClick:C},{default:y(()=>{var n;return[k(c(((n=t(e).options)==null?void 0:n.confirmText)||"Confirm"),1)]}),_:1},8,["variant"])])])])):M("",!0)])}}});export{H as P,L as T,R as X,A as _,I as u};
