# typeform-response-keeper
Typeform Response Keeper adalah service untuk menyimpan response dari Typeform dalam bentuk json. Semua attribut yang dikirimkan oleh Typeform akan disimpan, tanpa melakukan pemilihan attribut tertentu

### how-to
run GOOS=linux go build -o main, and then wrap main to main.zip, then upload it to aws lambda

### sample-json
Sample response json dari Typeform dapat ditemukan pada file response.json.sample. Penamaan file response merujuk pata attribut hidden di bawah form_response
