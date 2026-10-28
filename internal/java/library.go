package java

// libraryTypes are types httpgen recognises without their source: the JDK and Spring types that
// show up in controllers, resolvers and DTOs. Anything else from a library resolves to its
// written name and is treated as unknown.
var libraryTypes = setOf(
	// java.lang
	"java.lang.String", "java.lang.CharSequence", "java.lang.Object", "java.lang.Integer", "java.lang.Long",
	"java.lang.Short", "java.lang.Byte", "java.lang.Double", "java.lang.Float", "java.lang.Boolean",
	"java.lang.Character", "java.lang.Number", "java.lang.Void", "java.lang.Iterable", "java.lang.Enum",
	"java.lang.Record", "java.lang.Class", "java.lang.Throwable", "java.lang.Exception", "java.lang.RuntimeException",
	// java.util
	"java.util.List", "java.util.ArrayList", "java.util.LinkedList", "java.util.Set", "java.util.HashSet",
	"java.util.LinkedHashSet", "java.util.TreeSet", "java.util.SortedSet", "java.util.Collection",
	"java.util.Map", "java.util.HashMap", "java.util.LinkedHashMap", "java.util.TreeMap", "java.util.SortedMap",
	"java.util.Optional", "java.util.UUID", "java.util.Date", "java.util.Queue", "java.util.Deque",
	// java.time, java.math, java.sql
	"java.time.LocalDate", "java.time.LocalDateTime", "java.time.LocalTime", "java.time.Instant",
	"java.time.OffsetDateTime", "java.time.ZonedDateTime", "java.time.Duration", "java.time.YearMonth",
	"java.math.BigDecimal", "java.math.BigInteger", "java.sql.Date", "java.sql.Timestamp",
	// Spring
	"org.springframework.data.domain.Page", "org.springframework.data.domain.Slice",
	"org.springframework.data.domain.PageImpl", "org.springframework.data.domain.Pageable",
	"org.springframework.data.domain.Sort", "org.springframework.data.domain.Sort.Direction",
	"org.springframework.http.ResponseEntity",
	"org.springframework.http.HttpHeaders", "org.springframework.http.HttpStatus",
	"org.springframework.web.multipart.MultipartFile", "org.springframework.http.MediaType",
	"org.springframework.ui.Model", "org.springframework.validation.BindingResult",
	"org.springframework.web.context.request.WebRequest",
	"jakarta.servlet.http.HttpServletRequest", "jakarta.servlet.http.HttpServletResponse",
	"javax.servlet.http.HttpServletRequest", "javax.servlet.http.HttpServletResponse",
	"java.security.Principal",
	"io.leangen.graphql.execution.ResolutionEnvironment",
)

// LibraryEnums gives the constants of library enums that show up in DTOs.
var LibraryEnums = map[string][]string{
	"org.springframework.data.domain.Sort.Direction": {"ASC", "DESC"},
	"org.springframework.http.HttpStatus":            {"OK", "CREATED", "BAD_REQUEST", "NOT_FOUND"},
}

// libraryConstants are the library constants that commonly appear in mapping annotations.
var libraryConstants = map[string]string{
	"MediaType.APPLICATION_JSON_VALUE":                                     "application/json",
	"org.springframework.http.MediaType.APPLICATION_JSON_VALUE":            "application/json",
	"MediaType.APPLICATION_XML_VALUE":                                      "application/xml",
	"org.springframework.http.MediaType.APPLICATION_XML_VALUE":             "application/xml",
	"MediaType.MULTIPART_FORM_DATA_VALUE":                                  "multipart/form-data",
	"org.springframework.http.MediaType.MULTIPART_FORM_DATA_VALUE":         "multipart/form-data",
	"MediaType.APPLICATION_FORM_URLENCODED_VALUE":                          "application/x-www-form-urlencoded",
	"org.springframework.http.MediaType.APPLICATION_FORM_URLENCODED_VALUE": "application/x-www-form-urlencoded",
	"MediaType.TEXT_PLAIN_VALUE":                                           "text/plain",
	"org.springframework.http.MediaType.TEXT_PLAIN_VALUE":                  "text/plain",
	"MediaType.APPLICATION_OCTET_STREAM_VALUE":                             "application/octet-stream",
	"org.springframework.http.MediaType.APPLICATION_OCTET_STREAM_VALUE":    "application/octet-stream",
}

func setOf(items ...string) map[string]bool {
	m := make(map[string]bool, len(items))
	for _, s := range items {
		m[s] = true
	}
	return m
}
