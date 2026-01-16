	

#Область ПрограммныйИнтерфейс

// Функция возвращает структуру с данными свечей
// Источник: https://www.okx.com/docs-v5/en/?shell#order-book-trading-market-data-get-candlesticks
//
// Параметры:
//
//  instID - Строка - ID инструмента
//  bar - СправочникСсылка.big_BarTypes - размер баров (см. спр. )
//  after - timestamp
//  before - timestamp
//  limit - Число
//
// Возвращаемое значение: 
// The data returned will be arranged in an array like this: [ts,o,h,l,c,vol,volCcy,volCcyQuote,confirm]
Функция GET_Candles(RequestParameters) Экспорт
	
	requestPath	= "/api/v5/market/candles";	
	Возврат OKX_GET(RequestPath, RequestParameters);
	
КонецФункции 

Функция GET_CandlesHistory(RequestParameters) Экспорт
	
	requestPath	= "/api/v5/market/history-candles";	
	Возврат OKX_GET(RequestPath, RequestParameters);
	
КонецФункции

Функция GET_AccountBallance(Account) Экспорт

	requestPath	= "/api/v5/account/balance";
	Возврат OKX_GET(RequestPath,,Account);
	
КонецФункции
  
// Функция LoadInstruments
//
// Описание:
//
//
// Параметры (название, тип, дифференцированное значение)
//
// Возвращаемое значение: 
//
Функция GET_Instruments(RequestParameters, Account) Экспорт
	
	//requestPath	= "/api/v5/public/instruments";
	requestPath	= "/api/v5/account/instruments";
	Возврат OKX_GET(requestPath, RequestParameters, Account);
	
КонецФункции //LoadInstruments

Функция GET_DownloadCurrencies(RequestParameters, Account) Экспорт
	
	requestPath	= "/api/v5/asset/currencies";
	Возврат OKX_GET(requestPath, RequestParameters, Account);
	
КонецФункции

Функция GET_PendingOrders(RequestParameters, Account) Экспорт

	requestPath	= "/api/v5/trade/orders-pending";
	Возврат OKX_GET(requestPath, RequestParameters, Account);	
	
КонецФункции
  
Функция GET_AccountConfig(Account) Экспорт
		
	requestPath	= "/api/v5/account/config";
	Возврат OKX_GET(requestPath,, Account);
	
КонецФункции

Функция GET_OrderDetails(RequestParameters, Account) Экспорт
	
	requestPath	= "/api/v5/trade/order";
	Возврат OKX_GET(requestPath, RequestParameters, Account);	
		
КонецФункции

Функция POST_PlaceOrders(RequestParameters, Account) Экспорт
	
	requestPath	= "/api/v5/trade/order";
	Возврат OKX_POST(requestPath, RequestParameters, Account);
	
КонецФункции

Функция POST_AmendOrder(RequestParameters, Account) Экспорт

	requestPath	= "/api/v5/trade/amend-order";
	Возврат OKX_POST(requestPath, RequestParameters, Account);
	
КонецФункции

Функция GET_Positions(RequestParameters, Account) Экспорт
		
	requestPath	= "/api/v5/account/positions";
	Возврат OKX_GET(requestPath, RequestParameters, Account);	
		
КонецФункции

Функция POST_ClosePositions(RequestParameters, Account) Экспорт

	requestPath	= "/api/v5/trade/close-position";
	Возврат OKX_POST(requestPath, RequestParameters, Account);
	
КонецФункции

Функция GET_PositionHistory(RequestParameters, Account) Экспорт

	requestPath	= "/api/v5/account/positions-history";
	Возврат OKX_GET(requestPath, RequestParameters, Account);
	
КонецФункции 

Функция GET_OrdersHistory_7(RequestParameters, Account) Экспорт
	
	requestPath	= "/api/v5/trade/orders-history-archive";
	//requestPath	= "/api/v5/trade/orders-history";
	Возврат OKX_GET(requestPath, RequestParameters, Account);
	
КонецФункции

Функция POST_PlaceAlgoOrder(RequestParameters, Account) Экспорт

	requestPath	= "/api/v5/trade/order-algo";
	Возврат OKX_POST(requestPath, RequestParameters, Account);
	
КонецФункции

Функция POST_CancelAlgoOrder(RequestParameters, Account) Экспорт

	requestPath	= "/api/v5/trade/cancel-algos";
	Возврат OKX_POST(requestPath, RequestParameters, Account);
	
КонецФункции

Функция POST_AmendAlgoOrder(RequestParameters, Account) Экспорт

	requestPath	= "/api/v5/trade/amend-algos";
	Возврат OKX_POST(requestPath, RequestParameters, Account);
	
КонецФункции

Функция GET_AlgoOrderDetails(RequestParameters, Account) Экспорт
	
	requestPath	= "/api/v5/trade/order-algo";
	Возврат OKX_GET(requestPath, RequestParameters, Account);
	
КонецФункции 

Функция GET_AlgoOrderList(RequestParameters, Account) Экспорт
	
	requestPath	= "/api/v5/trade/orders-algo-pending";
	Возврат OKX_GET(requestPath, RequestParameters, Account);
	
КонецФункции

Функция GET_AlgoOrderHistory(RequestParameters, Account) Экспорт
	
	requestPath	= "/api/v5/trade/orders-algo-history";
	Возврат OKX_GET(requestPath, RequestParameters, Account);
	
КонецФункции

Функция GET_Price(RequestParameters) Экспорт

	requestPath	= "/api/v5/public/mark-price";
	Возврат OKX_GET(requestPath, RequestParameters);
	
КонецФункции

Функция GET_PriceTickers(RequestParameters) Экспорт

	requestPath	= "/api/v5/market/tickers";
	Возврат OKX_GET(requestPath, RequestParameters);
	
КонецФункции

Функция POST_SetLeverage(RequestParameters, Account) Экспорт

	requestPath	= "/api/v5/account/set-leverage";
	Возврат OKX_POST(requestPath, RequestParameters, Account);
	
КонецФункции

Функция POST_SetAccountMode(RequestParameters, Account) Экспорт

	requestPath	= "/api/v5/account/set-account-level";
	Возврат OKX_POST(requestPath, RequestParameters, Account);
	
КонецФункции

Функция POST_SetPositionMode(RequestParameters, Account) Экспорт

	requestPath	= "/api/v5/account/set-position-mode";
	Возврат OKX_POST(requestPath, RequestParameters, Account);
	
КонецФункции

Функция GET_Bills(RequestParameters, Account) Экспорт
	
	requestPath	= "/api/v5/account/bills";
	Возврат OKX_GET(requestPath, RequestParameters, Account);
	
КонецФункции

Функция GET_BillsThreeMonths(RequestParameters, Account) Экспорт
	
	requestPath	= "/api/v5/account/bills-archive";
	Возврат OKX_GET(requestPath, RequestParameters, Account);
	
КонецФункции

// Реализация метода GET /api/v5/market/tickers
Функция GET_Tickers(RequestParameters) Экспорт
	
    requestPath = "/api/v5/market/tickers";
    Возврат OKX_GET(requestPath, RequestParameters);
	
КонецФункции

#КонецОбласти


#Область СлужебныеПроцедурыИФункции

Функция OKX_POST(requestPath, RequestParameters, Account = Неопределено)

	Method	= "POST";
	Возврат OKX_Request(Method, RequestPath, RequestParameters, Account);
	
КонецФункции

Функция OKX_GET(RequestPath, RequestParameters = Неопределено, Account = Неопределено)
	
	Если RequestParameters = Неопределено Тогда
		RequestParameters = Новый Структура();		
	КонецЕсли; 
	
	Method	= "GET";
	Возврат OKX_Request(Method, RequestPath, RequestParameters, Account); 
	
КонецФункции

Функция OKX_Request(Method, RequestPath, RequestParameters, Account)
			
	URL 	= ПолучитьURLOKX();
	Adress 	= URL + RequestPath;
	
	Session = биг_КоннекторHTTP.СоздатьСессию();
	Session.Заголовки.Вставить("x-simulated-trading", ?(Account <> Неопределено И Account.demo, 1, 0));
	
	ПараметрыЗаписи 		= Новый Структура("ПереносСтрок", ПереносСтрокJSON.Unix);
	ДополнительныеПараметры = Новый Структура("ПараметрыЗаписиJSON", ПараметрыЗаписи);
		
	Если Account <> Неопределено Тогда		
		ЗаполнитьАвторизацию(Session, Method, RequestPath, RequestParameters, Account, ДополнительныеПараметры);	
	КонецЕсли;
	
	Если ВРег(Method) = "GET" Тогда
		Ответ	= биг_КоннекторHTTP.GetJson(Adress, RequestParameters,,Session);			
	Иначе		
		Ответ 	= биг_КоннекторHTTP.PostJson(Adress, RequestParameters, ДополнительныеПараметры, Session);	
	КонецЕсли;
		
	Возврат Ответ;
	
КонецФункции

Функция ПолучитьURLOKX()
	
	OKX = РегистрыСведений.биг_ПредопределенныеЭлементы.ПолучитьЗначениеПоИмени("OKX");
	Возврат OKX.domain;
	
КонецФункции

Процедура ЗаполнитьАвторизацию(Session, Method, endPoint, RequestParameters, Account, ДополнительныеПараметры)
	
	//Create a prehash string of timestamp + method + requestPath + body (where + represents String concatenation).
	timestamp	= биг_ОбщегоНазначенияКлиентСервер.СтрокаДатыСМиллисекундами();
	requestPath	= endPoint;
	
	Если Method = "GET" Тогда		
		requestPath = requestPath + ПараметрыЗапросаВСтроку(RequestParameters);
	КонецЕсли;
	
	body 					= ?(method = "POST", биг_КоннекторHTTP.ОбъектВJson(RequestParameters,,ДополнительныеПараметры.ПараметрыЗаписиJSON), "");
	prehashString			= timestamp + Method + requestPath + body;
	secretKey 				= Account.SecretKey;	
	secretKeyDoubleData		= ПолучитьДвоичныеДанныеИзСтроки(secretKey);
	sign 					= РаботаВМоделиСервисаБТС.Подпись(secretKeyDoubleData, prehashString);

	Session.Заголовки.Вставить("OK-ACCESS-KEY", 		Account.APIKey);
	Session.Заголовки.Вставить("OK-ACCESS-SIGN", 		sign);
	Session.Заголовки.Вставить("OK-ACCESS-PASSPHRASE", 	Account.passphrase);
	Session.Заголовки.Вставить("OK-ACCESS-TIMESTAMP", 	timestamp);
	
КонецПроцедуры
   
Функция ПараметрыЗапросаВСтроку(RequestParameters)
	
	СтрокаПараметры = "";
	
	Если RequestParameters <> Неопределено Тогда
		
		Для каждого Элемент Из RequestParameters Цикл
			Если СтрокаПараметры = "" Тогда
				СтрокаПараметры = СтрокаПараметры + "?";
			Иначе
				СтрокаПараметры = СтрокаПараметры + "&";	
			КонецЕсли;
			
			СтрокаПараметры = СтрокаПараметры + Элемент.Ключ + "=" + Элемент.Значение;
		КонецЦикла;	
		
	КонецЕсли;  
	
	Возврат СтрокаПараметры;
	
КонецФункции

#КонецОбласти	




