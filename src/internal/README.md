Para probar el programa ejecuta go run . en redcloud-files/src/internal
Luego sacas cualquier txt de examples y lo ubicas en /internal y puedes probar cualquier comando q hay por ahora 

OJO: esto es version beta 

Proximo paso definir una API HTTP con los handlers y el CLI solo es un cliente que hace peticiones a esa API.
Así, todo el core del sistema vive en la API (con SQLite, blobs, índices en memoria, etc.), y el CLI simplemente traduce comandos del usuario a requests HTTP.