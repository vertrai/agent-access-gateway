const $=id=>document.getElementById(id);
function adminHeaders(){const input=$('adminKey');const key=input?input.value.trim():sessionStorage.getItem('gatewayAdminKey')||'';if(input)sessionStorage.setItem('gatewayAdminKey',key);return {'Authorization':'Bearer '+key,'Content-Type':'application/json'}}
async function adminRequest(path,options={}){const response=await fetch(path,{...options,headers:{...adminHeaders(),...(options.headers||{})}});const data=await response.json().catch(()=>({}));if(!response.ok){const error=new Error(data.error||`HTTP ${response.status}`);error.status=response.status;error.data=data;throw error}return data}
function initAdminKey(){const input=$('adminKey');if(input)input.value=sessionStorage.getItem('gatewayAdminKey')||''}
function esc(value){return String(value??'').replace(/[&<>'"]/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;',"'":'&#39;','"':'&quot;'}[c]))}
function pill(status){return `<span class="pill ${esc(status)}">${esc(status||'-')}</span>`}
function setBusy(button,busy,text='处理中…'){if(!button)return;if(!button.dataset.label)button.dataset.label=button.textContent;button.disabled=busy;button.textContent=busy?text:button.dataset.label}
function showStatus(node,message,ok=false){node.textContent=message;node.className='status '+(ok?'success':'error')}
document.addEventListener('DOMContentLoaded',initAdminKey);
