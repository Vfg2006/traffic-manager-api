package main

import (
	"database/sql"
	"log"
	"os"
	"time"

	_ "github.com/lib/pq"
	gonanoid "github.com/matoous/go-nanoid/v2"
)

const (
	// dbConnectionString = "postgresql://traffic_user:7xYhIk2ek9sER6ZpNCbieKZH1Oadsmd7@dpg-cv0thsgfnakc738l80cg-a.virginia-postgres.render.com/traffic_81cm"
	dbConnectionString = "postgresql://postgres:root@localhost:5432/traffic?sslmode=disable"
	idLength           = 6
	characters         = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
)

type Business struct {
	ExternalID string
	Name       string
	Origin     string
}

type Account struct {
	ExternalID         string
	Name               string
	Nickname           string
	ExternalBusinessID string
	Origin             string
}

func setupLogger() {
	// Configura o logger para incluir data, hora e arquivo
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
	log.Println("Iniciando script de migração...")
}

func generateID() string {
	id, _ := gonanoid.Generate(characters, idLength)
	return id
}

func insertBusiness(tx *sql.Tx, businessList []Business) map[string]string {
	log.Printf("Iniciando inserção de %d business managers...", len(businessList))
	startTime := time.Now()

	stmt, err := tx.Prepare(`INSERT INTO business_manager (id, external_id, name, origin) VALUES ($1, $2, $3, $4) RETURNING id`)
	if err != nil {
		log.Fatalf("ERRO ao preparar statement para business_manager: %v", err)
	}
	defer stmt.Close()

	businessMap := make(map[string]string)
	successCount := 0
	errorCount := 0

	for i, b := range businessList {
		id := generateID()
		_, err := stmt.Exec(id, b.ExternalID, b.Name, b.Origin)
		if err != nil {
			log.Printf("ERRO ao inserir business [%d/%d] %s: %v", i+1, len(businessList), b.Name, err)
			errorCount++
			continue
		}
		businessMap[b.ExternalID] = id
		successCount++
		if i > 0 && i%10 == 0 {
			log.Printf("Progresso: %d/%d business processados", i+1, len(businessList))
		}
	}

	elapsed := time.Since(startTime)
	log.Printf("Inserção de business concluída em %v. Sucesso: %d, Erros: %d", elapsed, successCount, errorCount)

	return businessMap
}

func insertAccounts(tx *sql.Tx, accountList []Account, businessMap map[string]string) {
	log.Printf("Iniciando inserção de %d contas...", len(accountList))
	startTime := time.Now()

	stmt, err := tx.Prepare(`INSERT INTO accounts (id, external_id, name, nickname, business_id, origin) VALUES ($1, $2, $3, $4, $5, $6)`)
	if err != nil {
		log.Fatalf("ERRO ao preparar statement para accounts: %v", err)
	}
	defer stmt.Close()

	successCount := 0
	errorCount := 0
	businessNotFoundCount := 0

	for i, a := range accountList {
		id := generateID()
		businessID, exists := businessMap[a.ExternalBusinessID]
		if !exists {
			log.Printf("AVISO: Business não encontrado para conta %s (External ID: %s)", a.Name, a.ExternalID)
			businessNotFoundCount++
			continue
		}

		_, err := stmt.Exec(id, a.ExternalID, a.Name, a.Nickname, businessID, a.Origin)
		if err != nil {
			log.Printf("ERRO ao inserir account [%d/%d] %s: %v", i+1, len(accountList), a.Name, err)
			errorCount++
			continue
		}
		successCount++
		if i > 0 && i%10 == 0 {
			log.Printf("Progresso: %d/%d contas processadas", i+1, len(accountList))
		}
	}

	elapsed := time.Since(startTime)
	log.Printf("Inserção de contas concluída em %v. Sucesso: %d, Erros: %d, Business não encontrados: %d",
		elapsed, successCount, errorCount, businessNotFoundCount)
}

func addUniqueConstraintToStoreRanking(db *sql.DB) {
	log.Println("Adicionando constraint UNIQUE na coluna account_id da tabela store_ranking...")

	// Verificar se a constraint já existe
	var constraintExists bool
	err := db.QueryRow(`
		SELECT EXISTS (
			SELECT 1 FROM information_schema.table_constraints 
			WHERE table_name = 'store_ranking' 
			AND constraint_type = 'UNIQUE' 
			AND constraint_name LIKE '%account_id%'
		)
	`).Scan(&constraintExists)
	if err != nil {
		log.Printf("ERRO ao verificar constraint existente: %v", err)
		return
	}

	if constraintExists {
		log.Println("Constraint UNIQUE já existe na coluna account_id da tabela store_ranking")
		return
	}

	// Adicionar a constraint UNIQUE
	_, err = db.Exec("ALTER TABLE store_ranking ADD CONSTRAINT store_ranking_account_id_unique UNIQUE (account_id)")
	if err != nil {
		log.Printf("ERRO ao adicionar constraint UNIQUE: %v", err)
		return
	}

	log.Println("Constraint UNIQUE adicionada com sucesso na coluna account_id da tabela store_ranking")
}

func addMonthFieldToStoreRanking(db *sql.DB) {
	log.Println("Adicionando campo month na tabela store_ranking...")

	// Verificar se a coluna month já existe
	var columnExists bool
	err := db.QueryRow(`
		SELECT EXISTS (
			SELECT 1 FROM information_schema.columns 
			WHERE table_name = 'store_ranking' 
			AND column_name = 'month'
		)
	`).Scan(&columnExists)
	if err != nil {
		log.Printf("ERRO ao verificar coluna month existente: %v", err)
		return
	}

	if columnExists {
		log.Println("Coluna month já existe na tabela store_ranking")
		return
	}

	// Adicionar a coluna month
	_, err = db.Exec("ALTER TABLE store_ranking ADD COLUMN month VARCHAR(7)")
	if err != nil {
		log.Printf("ERRO ao adicionar coluna month: %v", err)
		return
	}

	// Definir valor padrão para registros existentes (mês atual)
	_, err = db.Exec(`
		UPDATE store_ranking 
		SET month = TO_CHAR(CURRENT_DATE, 'MM-YYYY')
		WHERE month IS NULL
	`)
	if err != nil {
		log.Printf("ERRO ao definir valor padrão para coluna month: %v", err)
		return
	}

	// Tornar a coluna NOT NULL
	_, err = db.Exec("ALTER TABLE store_ranking ALTER COLUMN month SET NOT NULL")
	if err != nil {
		log.Printf("ERRO ao tornar coluna month NOT NULL: %v", err)
		return
	}

	// Remover constraint antiga e adicionar nova constraint composta
	_, err = db.Exec("ALTER TABLE store_ranking DROP CONSTRAINT IF EXISTS store_ranking_account_id_unique")
	if err != nil {
		log.Printf("AVISO: Não foi possível remover constraint antiga: %v", err)
	}

	// Adicionar nova constraint composta (account_id, month)
	_, err = db.Exec("ALTER TABLE store_ranking ADD CONSTRAINT store_ranking_account_month_unique UNIQUE (account_id, month)")
	if err != nil {
		log.Printf("ERRO ao adicionar constraint composta: %v", err)
		return
	}

	log.Println("Campo month adicionado com sucesso na tabela store_ranking")
}

func main() {
	setupLogger()
	log.Println("Conectando ao banco de dados...")

	db, err := sql.Open("postgres", dbConnectionString)
	if err != nil {
		log.Fatalf("ERRO ao conectar ao banco de dados: %v", err)
	}
	defer db.Close()

	// Verificar conexão
	err = db.Ping()
	if err != nil {
		log.Fatalf("ERRO ao verificar conexão com o banco: %v", err)
	}
	log.Println("Conexão com o banco de dados estabelecida com sucesso")

	// Adicionar constraint UNIQUE na tabela store_ranking
	addUniqueConstraintToStoreRanking(db)

	// Adicionar campo month na tabela store_ranking
	addMonthFieldToStoreRanking(db)

	businessList := []Business{
		{"8966560533455226", "Instituto Visão Solidária", "meta"},
		{"8028251353910984", "ivs.corumba1", "meta"},
		{"6053046984822862", "ivsfranciscobeltrao", "meta"},
		{"3697479543902959", "Instituto Visão Solidária Arapiraca-AL", "meta"},
		{"3571531246414718", "ivs.santoandre.sp", "meta"},
		{"2205702143163535", "ivs.concordia", "meta"},
		{"1853333271865946", "Instituto Visão Solidária Blumenau-SC", "meta"},
		{"1633707287033892", "BM - IVS Florianópolis Loja 02", "meta"},
		{"1437940213680235", "BM - IVS São Luís", "meta"},
		{"1313443652579781", "Ivs São Luis 01", "meta"},
		{"1224602135453663", "Instituto Visão Solidária Castro-PR", "meta"},
		{"1219907072600020", "Instituto Visão Solidária Campo Mourão-PR", "meta"},
		{"1084530362897150", "ivsrioverde", "meta"},
		{"915347106672198", "Instituto Visão Solidária Guarapuava-PR", "meta"},
		{"876586267809329", "Shopping da Visão", "meta"},
		{"873827160934226", "BM - IVS Curitiba-PR", "meta"},
		{"862151325374240", "Instituto Visão Solidária Recife-PE", "meta"},
		{"827405099216910", "Instituto Visão Solidária Santa Inês-MA", "meta"},
		{"773596444719372", "Instituto Visão Solidária Tubarão-SC", "meta"},
		{"711575440623930", "ivscaceres", "meta"},
		{"698408995302760", "Ivs Central 02", "meta"},
		{"633532601642022", "BM - IVS Lojas", "meta"},
		{"596311689784738", "Instituto Visão Solidária Curitiba-PR Loja 4", "meta"},
		{"515537170639501", "Instituto Visão Solidária Inhumas", "meta"},
		{"481374586437098", "Instituto Visão solidária Varzea Grande", "meta"},
		{"436747232124707", "ivs.chapeco1", "meta"},
		{"396910919668820", "Rinely Soares", "meta"},
		{"372763745236383", "BM - IVS Maceió", "meta"},
		{"302112065442822", "IONC", "meta"},
		{"276318415018603", "Mercadao dos Oculos Cuiaba/MT", "meta"},
		{"266805339455599", "ivstreslagoas", "meta"},
		{"136708522651942", "BM - IVS Tangara Av Brasil", "meta"},
		{"123375432529288", "IVS Central", "meta"},
		{"100914711246610", "bruno jacomeli", "meta"},
	}
	log.Printf("Total de %d business managers definidos para inserção", len(businessList))

	accountList := []Account{
		{"1444838296485002", "IVS RIO BRANCO", "IVS RIO BRANCO", "8966560533455226", "meta"},
		{"1863484354144119", "IVS CORUMBÁ", "IVS CORUMBÁ", "8028251353910984", "meta"},
		{"1634571304057374", "IVS CONCÓRDIA", "IVS CONCÓRDIA", "6053046984822862", "meta"},
		{"1409588352945215", "IVS BELTRÃO", "IVS BELTRÃO", "6053046984822862", "meta"},
		{"2265113900502499", "IVS ARAPIRACA 01", "IVS ARAPIRACA 01", "3697479543902959", "meta"},
		{"1005456757545175", "IVS SANTO ANDRÉ", "IVS SANTO ANDRÉ", "3571531246414718", "meta"},
		{"1252881642374320", "ivs.concordia", "ivs.concordia", "2205702143163535", "meta"},
		{"1299583051203676", "IVS BLUMENAU SC", "IVS BLUMENAU SC", "1853333271865946", "meta"},
		{"1346282669721390", "IVS CURITIBA 04", "IVS CURITIBA 04", "1633707287033892", "meta"},
		{"2595929670592659", "IVS TAGUATINGA", "IVS TAGUATINGA", "1633707287033892", "meta"},
		{"1516950725626910", "IVS FORMOSA", "IVS FORMOSA", "1633707287033892", "meta"},
		{"1119639459869991", "IVS ANÁPOLIS", "IVS ANÁPOLIS", "1633707287033892", "meta"},
		{"1984957985324515", "IVS ALVORADA", "IVS ALVORADA", "1633707287033892", "meta"},
		{"1083972926439970", "IVS ITAPEMA", "IVS ITAPEMA", "1633707287033892", "meta"},
		{"919177616819612", "IVS SÃO BERNARDO DO CAMPO", "IVS SÃO BERNARDO DO CAMPO", "1633707287033892", "meta"},
		{"585332004163205", "IVS CASTRO", "IVS CASTRO", "1633707287033892", "meta"},
		{"1750853189071646", "IVS CAMPO MOURÃO", "IVS CAMPO MOURÃO", "1633707287033892", "meta"},
		{"803354568519284", "IVS FLORIPA 02", "IVS FLORIPA 02", "1633707287033892", "meta"},
		{"3936347756646990", "CLINICA PONTO VISION", "CLINICA PONTO VISION", "1437940213680235", "meta"},
		{"7218260008290991", "IVS SÃO LUIS 04", "IVS SÃO LUIS 04", "1437940213680235", "meta"},
		{"354916653595268", "IVS São Luis 03", "IVS São Luis 03", "1437940213680235", "meta"},
		{"3619750884910605", "IVS SÃO LUIS 02 (NOVA)", "IVS SÃO LUIS 02 (NOVA)", "1437940213680235", "meta"},
		{"1254155315270996", "IVS CASTANHAL", "IVS CASTANHAL", "1313443652579781", "meta"},
		{"596529965796690", "IVS SÃO LUIS 01 (ANTIGA)", "IVS SÃO LUIS 01 (ANTIGA)", "1313443652579781", "meta"},
		{"1302117490697931", "IVS CASTRO PR", "IVS CASTRO PR", "1224602135453663", "meta"},
		{"546263314685829", "IVS CAMPO MOURÃO PR", "IVS CAMPO MOURÃO PR", "1219907072600020", "meta"},
		{"981631466709615", "IVS RONDONÓPOLIS MARECHAL", "IVS RONDONÓPOLIS MARECHAL", "1084530362897150", "meta"},
		{"1427721931436068", "IVS RIO VERDE", "IVS RIO VERDE", "1084530362897150", "meta"},
		{"1319152176018806", "IVS CASCAVEL", "IVS CASCAVEL", "915347106672198", "meta"},
		{"2950255985128156", "IVS UNIÃO DA VITÓRIA", "IVS UNIÃO DA VITÓRIA", "915347106672198", "meta"},
		{"1130394065275474", "IVS MEDIANEIRA", "IVS MEDIANEIRA", "915347106672198", "meta"},
		{"1533315867258708", "ivs guarapuava", "ivs guarapuava", "915347106672198", "meta"},
		{"324421557065219", "Shopping da visão", "Shopping da visão", "876586267809329", "meta"},
		{"1855692531519794", "IVS Curitiba 02", "IVS Curitiba 02", "873827160934226", "meta"},
		{"880081287237456", "IVS Curitiba 01", "IVS Curitiba 01", "873827160934226", "meta"},
		{"2504639926396233", "IVS RECIFE 01", "IVS RECIFE 01", "862151325374240", "meta"},
		{"1593258204653291", "IVS BACABAL", "IVS BACABAL", "827405099216910", "meta"},
		{"950946883576119", "CONTA RESERVA", "CONTA RESERVA", "827405099216910", "meta"},
		{"995509252312137", "IVS SANTA INÊS", "IVS SANTA INÊS", "827405099216910", "meta"},
		{"1557394574849852", "IVS TUBARÃO 01", "IVS TUBARÃO 01", "773596444719372", "meta"},
		{"998358398823075", "IVS CAMPO NOVO DO PARECIS-MT", "IVS CAMPO NOVO DO PARECIS-MT", "711575440623930", "meta"},
		{"929435455299422", "IVS CÁCERES", "IVS CÁCERES", "711575440623930", "meta"},
		{"579596595036930", "IVS FLORIPA 03", "IVS FLORIPA 03", "698408995302760", "meta"},
		{"454727507514585", "IVS CPA - CUIABÁ", "IVS CPA - CUIABÁ", "698408995302760", "meta"},
		{"8816526641764533", "IVS BLUMENAU - SC", "IVS BLUMENAU - SC", "698408995302760", "meta"},
		{"553459590617146", "IVS ALTA FLORESTA", "IVS ALTA FLORESTA", "698408995302760", "meta"},
		{"472243512383070", "IVS CAMPO GRANDE", "IVS CAMPO GRANDE", "698408995302760", "meta"},
		{"408153855137780", "IVS FLORIPA 01", "IVS FLORIPA 01", "698408995302760", "meta"},
		{"291745000601431", "IVS JUÍNA 01", "IVS JUÍNA 01", "698408995302760", "meta"},
		{"1430515687540084", "IVS LONDRINA 01", "IVS LONDRINA 01", "698408995302760", "meta"},
		{"1070091404435016", "IVS VILHENA", "IVS VILHENA", "698408995302760", "meta"},
		{"1380229756085002", "IVS DOURADOS", "IVS DOURADOS", "698408995302760", "meta"},
		{"3217796161846810", "IVS RESERVA", "IVS RESERVA", "633532601642022", "meta"},
		{"1207738506612031", "IVS PATO BRANCO", "IVS PATO BRANCO", "633532601642022", "meta"},
		{"115053504888367", "RONDONOPOLIS MARECHAL (FRANQUEADO)", "RONDONOPOLIS MARECHAL (FRANQUEADO)", "633532601642022", "meta"},
		{"176582088551730", "IVS LOJA CACOAL", "IVS LOJA CACOAL", "633532601642022", "meta"},
		{"1207826223429060", "IVS LOJA PRIMAVERA 01", "IVS LOJA PRIMAVERA 01", "633532601642022", "meta"},
		{"571583111533274", "IVS JÍ PARANÁ", "IVS JÍ PARANÁ", "633532601642022", "meta"},
		{"5994902080590291", "IVS LOJAS 02", "IVS LOJAS 02", "633532601642022", "meta"},
		{"484943487027641", "IVS LOJAS 01", "IVS LOJAS 01", "633532601642022", "meta"},
		{"3228762980612269", "IVS CURITIBA 04", "IVS CURITIBA 04", "596311689784738", "meta"},
		{"599378003087162", "IVS - RIO BRANCO", "IVS - RIO BRANCO", "515537170639501", "meta"},
		{"1175949253886576", "IVS RIO VERDE", "IVS RIO VERDE", "515537170639501", "meta"},
		{"1603093350533943", "IVS SOBRAL 01", "IVS SOBRAL 01", "515537170639501", "meta"},
		{"284402224075896", "284402224075896", "284402224075896", "515537170639501", "meta"},
		{"646465077112286", "IVS GÔIANIA 01", "IVS GÔIANIA 01", "515537170639501", "meta"},
		{"525557093489431", "IVS VARZEA GRANDE", "IVS VARZEA GRANDE", "481374586437098", "meta"},
		{"1501377093850749", "CA - IVS CHAPECO 2", "CA - IVS CHAPECO 2", "436747232124707", "meta"},
		{"376157638636960", "IVS CHAPECÓ", "IVS CHAPECÓ", "436747232124707", "meta"},
		{"296002669681680", "AD01", "AD01", "396910919668820", "meta"},
		{"1482083339187994", "IVS MACEIÓ", "IVS MACEIÓ", "372763745236383", "meta"},
		{"3333130986919456", "IONC TANGÁRA", "IONC TANGÁRA", "302112065442822", "meta"},
		{"866406157609042", "IONC RONDONOPÓLIS", "IONC RONDONOPÓLIS", "302112065442822", "meta"},
		{"1733293457063355", "IVS LONDRINA", "IVS LONDRINA", "302112065442822", "meta"},
		{"1489011574836948", "CA - Mercadão dos Óculos", "CA - Mercadão dos Óculos", "276318415018603", "meta"},
		{"1002995995220444", "CLINICA DE OLHOS VISÃO", "CLINICA DE OLHOS VISÃO", "266805339455599", "meta"},
		{"1070490861076333", "IVS JARU", "IVS JARU", "266805339455599", "meta"},
		{"512952071682517", "IVS ARIQUEMES 01", "IVS ARIQUEMES 01", "266805339455599", "meta"},
		{"234852009719110", "IVS ROLIM DE MOURA", "IVS ROLIM DE MOURA", "266805339455599", "meta"},
		{"311486921289137", "IVS TRÊS LAGOAS", "IVS TRÊS LAGOAS", "266805339455599", "meta"},
		{"2323269388033649", "IVS CRICIÚMA", "IVS CRICIÚMA", "136708522651942", "meta"},
		{"2027755881011889", "IVS SÃO JOSÉ", "IVS SÃO JOSÉ", "136708522651942", "meta"},
		{"776149077605250", "IVS - TANGARA DA SERRA", "IVS - TANGARA DA SERRA", "136708522651942", "meta"},
		{"1479853520073498", "IVS PASSO FUNDO", "IVS PASSO FUNDO", "123375432529288", "meta"},
		{"375867638533249", "IVS TAUBATE 02", "IVS TAUBATE 02", "123375432529288", "meta"},
		{"1439983843219130", "IVS ERECHIM", "IVS ERECHIM", "123375432529288", "meta"},
		{"731514455491103", "IVS Nova Iguaçu", "IVS Nova Iguaçu", "123375432529288", "meta"},
		{"1544878579583034", "IVS TAUBATE 01", "IVS TAUBATE 01", "123375432529288", "meta"},
		{"1500923160758086", "LUAN - PESSOAL", "LUAN - PESSOAL", "123375432529288", "meta"},
		{"332042599168792", "IVS SANTO ANDRÉ", "IVS SANTO ANDRÉ", "123375432529288", "meta"},
		{"602028022065769", "IVS SINOP", "IVS SINOP", "123375432529288", "meta"},
		{"1227514148112976", "Ivs Franquias [boleto]", "Ivs Franquias [boleto]", "123375432529288", "meta"},
		{"504940388207713", "IVS FRANQUIA", "IVS FRANQUIA", "123375432529288", "meta"},
		{"304077993712690", "CONTA 01", "CONTA 01", "100914711246610", "meta"},
	}
	log.Printf("Total de %d contas definidas para inserção", len(accountList))

	startTime := time.Now()
	log.Println("Iniciando transação...")

	tx, err := db.Begin()
	if err != nil {
		log.Fatalf("ERRO ao iniciar transação: %v", err)
	}

	businessMap := insertBusiness(tx, businessList)
	log.Printf("Mapeados %d business managers com sucesso", len(businessMap))

	insertAccounts(tx, accountList, businessMap)

	if err := tx.Commit(); err != nil {
		log.Printf("ERRO ao confirmar transação: %v", err)
		if err := tx.Rollback(); err != nil {
			log.Fatalf("ERRO ao reverter transação: %v", err)
		}
		log.Println("Transação revertida")
		os.Exit(1)
	}

	elapsed := time.Since(startTime)
	log.Printf("Carga inicial concluída em %v!", elapsed)
}
